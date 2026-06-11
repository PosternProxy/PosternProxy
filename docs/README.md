# PosternProxy Documentation Site

The documentation site is built with [Hugo](https://gohugo.io/) and the
[Hugo Book](https://github.com/alex-shpak/hugo-book) theme.

## Requirements

- Hugo Extended 0.158.0 or newer
- Go 1.25 or newer
- Git

## Local development

On Windows, install a compatible Extended build from an Administrator
PowerShell session:

```powershell
choco uninstall hugo hugo-extended -y
choco install hugo-extended --version=0.163.0 -y
hugo version
```

Do not install the `hugo` and `hugo-extended` Chocolatey packages together;
they both provide `hugo.exe`, and the older package can win on `PATH`.

Run these commands from this directory:

```sh
hugo server -D
```

Hugo downloads the pinned theme module on the first run and serves the site at
`http://localhost:1313/`.

To create a production build:

```sh
hugo --gc --minify
```

The generated site is written to `public/` and is not committed.
