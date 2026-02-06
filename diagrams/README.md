# Diagrams

Place `.excalidraw` files here. A GitHub Action automatically exports them to
light and dark SVGs in `exported/`.

## Expected files

| Source file | Used in README as |
|---|---|
| `architecture.excalidraw` | Architecture overview |
| `sequence-tenant-creation.excalidraw` | Tenant creation flow |
| `sequence-sftp-auth.excalidraw` | SFTP authentication hook |
| `sequence-csv-processing.excalidraw` | CSV upload and processing |

## How it works

1. Create or edit a `.excalidraw` file in this directory
2. Push to `main`
3. The `excalidraw-export` workflow runs and generates:
   - `exported/<name>-light.svg`
   - `exported/<name>-dark.svg`
4. The README uses `<picture>` tags to show the right theme automatically
