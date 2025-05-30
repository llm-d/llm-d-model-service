# API Reference

Refer to the [`api_reference`](./api_reference/out.html) folder.

# Command to build API Reference

You can generate the API reference documentation by running:

```sh
make api-docs
```

This command will:
1. Generate asciidoc using [crd-ref-docs](https://github.com/elastic/crd-ref-docs)
2. Convert the asciidoc to HTML using [asciidoctor](https://asciidoctor.org/)
