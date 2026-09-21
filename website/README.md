# svg2drawio website

The single-page site behind <https://svg2drawio.runlocal.dev>. Hand-rolled
HTML/CSS — no SSG, no framework, no JS dependencies. Served as static files from
a GCS bucket.

Sibling sites using the same pattern: `../../claude-status-macos-menu-bar/website/`
and `../../klimax-website/`.

## Layout

```
website/
├── index.html      # the landing page
├── styles.css      # all styles; light/dark via prefers-color-scheme
├── 404.html
├── robots.txt · sitemap.xml
├── assets/
│   ├── favicon.svg
│   ├── og-image.svg   # editable source for the social card (1200×630)
│   └── og-image.png   # rendered card the crawlers fetch
└── cicd/
    └── setup-gcp-wif.sh   # one-time GCP setup (idempotent), not deployed
```

## Deploy

`.github/workflows/deploy-website.yaml` mirrors `website/` into
`gs://svg2drawio-runlocal-dev` on every push to `main` that touches this
directory. It authenticates with Workload Identity Federation — no service
account keys. `cicd/` is excluded from the rsync.

The bucket is not domain-named: `svg2drawio.runlocal.dev` is served by the
shared HTTPS load balancer managed in the `gcp-load-balancer-bco` repo (backend
bucket, managed certificate, host rule), which references the bucket by name.

Run `cicd/setup-gcp-wif.sh` once to create the bucket, the service account and
the repo binding.

## Regenerating the social card

```bash
rsvg-convert -w 1200 -h 630 assets/og-image.svg -o assets/og-image.png
```
