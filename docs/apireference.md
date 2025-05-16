# API Reference

Refer to the [`api_reference`](./api_reference/out.html) folder.

# Command to build API Reference

- Generate asciidoc using [crd-ref-docs](https://github.com/elastic/crd-ref-docs)

```sh
crd-ref-docs --source-path=./api/v1alpha1  --config=./docs/api_reference/config.yaml --output-path=./docs/api_reference
```

- Convert asciidoc to HTML by installing [asciidoctor](https://asciidoctor.org/)

```sh
asciidoctor ./docs/api_reference/out.asciidoc
```