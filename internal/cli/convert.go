package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bcollard/svg2drawio/internal/convert"
	"github.com/bcollard/svg2drawio/internal/mxgraph"
	"github.com/bcollard/svg2drawio/internal/stencil"
	"github.com/spf13/cobra"
)

// convertOptions holds the flags shared by the conversion commands.
type convertOptions struct {
	out      string
	decimals int
	scale    float64
	noGroups bool
	noEdges  bool
	name     string
	quiet    bool
}

func (o *convertOptions) addFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVarP(&o.out, "out", "o", "", "Output file or directory; - writes to stdout")
	f.IntVar(&o.decimals, "decimals", 2, "Round coordinates to this many decimals (-1 keeps full precision)")
	f.Float64Var(&o.scale, "scale", 1, "Scale all coordinates by this factor")
	f.BoolVar(&o.noGroups, "no-groups", false, "Flatten SVG groups instead of keeping draw.io groups")
	f.BoolVar(&o.noEdges, "no-edges", false, "Convert lines and polylines to shapes instead of edges")
	f.StringVar(&o.name, "name", "", "Name of the diagram page or the stencil library")
	f.BoolVarP(&o.quiet, "quiet", "q", false, "Do not print what was written")
}

func (o *convertOptions) convertOptions() convert.Options {
	opt := convert.DefaultOptions()
	opt.Decimals = o.decimals
	opt.Scale = o.scale
	opt.Groups = !o.noGroups
	opt.Edges = !o.noEdges
	return opt
}

func newConvertCmd() *cobra.Command {
	opts := &convertOptions{}
	cmd := &cobra.Command{
		Use:     "convert [flags] <input.svg|directory>...",
		Aliases: []string{"diagram"},
		Short:   "Convert SVGs to .drawio diagrams (the default action)",
		Long: `Convert SVG files into .drawio diagrams made of native draw.io cells.

This is what the bare svg2drawio command does; the subcommand exists so scripts
can be explicit.`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.runDiagram(cmd, args)
		},
	}
	opts.addFlags(cmd)
	return cmd
}

func newStencilCmd() *cobra.Command {
	opts := &convertOptions{}
	cmd := &cobra.Command{
		Use:     "stencil [flags] <input.svg|directory>...",
		Aliases: []string{"library"},
		Short:   "Convert SVGs to an mxGraph stencil library",
		Long: `Convert SVG files into a single mxGraph stencil library (<shapes>), one
shape per input file.

Load the result in draw.io with Extras > Edit Shape Library, or host it and add
it as a custom library. This is the output format of the original Java svg2xml.`,
		Example: `  svg2drawio stencil -o icons.xml icons/
  svg2drawio stencil --name mesh -o mesh.xml a.svg b.svg`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.runStencil(cmd, args)
		},
	}
	opts.addFlags(cmd)
	return cmd
}

// runDiagram writes one .drawio per input, or a single multi-page file when the
// output names one file and several inputs are given.
func (o *convertOptions) runDiagram(cmd *cobra.Command, args []string) error {
	files, err := collectSVGs(args)
	if err != nil {
		return err
	}
	opt := o.convertOptions()

	multiPage := o.out != "" && o.out != "-" && len(files) > 1 && !looksLikeDir(o.out)
	if o.out == "-" || multiPage {
		out := &mxgraph.File{}
		for _, f := range files {
			file, err := o.diagramOf(f, opt)
			if err != nil {
				return err
			}
			out.Models = append(out.Models, file.Models...)
		}
		return o.write(cmd, o.out, out.XML(), "")
	}

	for _, f := range files {
		file, err := o.diagramOf(f, opt)
		if err != nil {
			return err
		}
		dest := destFor(o.out, f, files, ".drawio")
		if err := o.write(cmd, dest, file.XML(), f); err != nil {
			return err
		}
	}
	return nil
}

func (o *convertOptions) diagramOf(path string, opt convert.Options) (*mxgraph.File, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	opt.PageName = o.name
	file, err := convert.Diagram(r, path, opt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return file, nil
}

// runStencil writes every input as one shape of a single stencil library.
func (o *convertOptions) runStencil(cmd *cobra.Command, args []string) error {
	files, err := collectSVGs(args)
	if err != nil {
		return err
	}
	opt := o.convertOptions()

	lib := &stencil.Library{Name: o.name}
	if lib.Name == "" {
		lib.Name = libraryName(files)
	}

	for _, f := range files {
		r, err := os.Open(f)
		if err != nil {
			return err
		}
		opt.ShapeName = shapeName(lib.Name, f)
		sh, err := convert.StencilShape(r, f, opt)
		r.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		lib.Shapes = append(lib.Shapes, sh)
	}

	dest := o.out
	if dest == "" {
		dest = lib.Name + ".xml"
	}
	if looksLikeDir(dest) {
		dest = filepath.Join(dest, lib.Name+".xml")
	}
	return o.write(cmd, dest, lib.XML(), strings.Join(files, ", "))
}

func (o *convertOptions) write(cmd *cobra.Command, dest, content, source string) error {
	if dest == "-" || dest == "" {
		_, err := io.WriteString(cmd.OutOrStdout(), content)
		return err
	}
	if dir := filepath.Dir(dest); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return err
	}
	if !o.quiet {
		if source != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s\n", source, dest)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", dest)
		}
	}
	return nil
}

// shapeName builds the "library.shape" name draw.io shows in the shape picker.
func shapeName(lib, path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.ToLower(lib) + "." + base
}

func libraryName(files []string) string {
	if len(files) == 1 {
		return strings.TrimSuffix(filepath.Base(files[0]), filepath.Ext(files[0]))
	}
	dir := filepath.Base(filepath.Dir(files[0]))
	if dir == "" || dir == "." || dir == string(filepath.Separator) {
		return "stencils"
	}
	return dir
}

// destFor resolves where a converted file is written. With several inputs the
// output is a directory, and every file lands in it.
func destFor(out, input string, all []string, ext string) string {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)) + ext
	if out == "" {
		return filepath.Join(filepath.Dir(input), base)
	}
	if len(all) == 1 && !looksLikeDir(out) {
		return out
	}
	return filepath.Join(out, base)
}

// looksLikeDir decides whether an output path names a directory. A path that
// exists as one, ends with a separator, or carries no file extension is taken
// as a directory; anything else is a file.
func looksLikeDir(p string) bool {
	if p == "" || p == "-" {
		return false
	}
	if strings.HasSuffix(p, string(filepath.Separator)) {
		return true
	}
	if info, err := os.Stat(p); err == nil {
		return info.IsDir()
	}
	return filepath.Ext(p) == ""
}

// collectSVGs expands the inputs, walking directories for .svg files.
func collectSVGs(inputs []string) ([]string, error) {
	var out []string
	for _, in := range inputs {
		info, err := os.Stat(in)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, in)
			continue
		}
		var found []string
		err = filepath.WalkDir(in, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".svg") {
				found = append(found, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(found)
		out = append(out, found...)
	}
	if len(out) == 0 {
		return nil, errors.New("no .svg files found")
	}
	return out, nil
}
