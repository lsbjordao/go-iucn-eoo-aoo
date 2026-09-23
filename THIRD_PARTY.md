# Dependencies and acknowledgements

- `github.com/airbusgeo/godal` v0.0.18 — Airbus, Apache-2.0. [Code/license](https://github.com/airbusgeo/godal/tree/v0.0.18).
- GDAL/OGR — native geospatial library. [License](https://gdal.org/en/stable/license.html).
- PROJ — native coordinate-transformation library. [License](https://proj.org/en/stable/about.html#license).
- MapLibre GL JS — loaded at runtime by generated HTML reports; not embedded in the Go binary. [License](https://github.com/maplibre/maplibre-gl-js/blob/main/LICENSE.txt).
- IUCN guidelines and tool documentation — methodological references only; no incorporated code, institutional partnership, certification, or endorsement is implied.

Go dependencies are not vendored. `go.sum` pins module checksums.

Linux and macOS release binaries use GDAL/PROJ runtime libraries supplied by the target operating system or Homebrew. Those native libraries are not copied into the Unix release tarballs.

The Windows portable ZIP is different: it bundles the runtime DLL dependency closure required by the UCRT64 build, together with GDAL and PROJ data directories. The release workflow records the MSYS2 packages that own those DLLs and copies their available license files into the release bundle. The bundled files retain their original upstream licenses; inclusion in this repository's release archive does not relicense them under MIT.

Any future package format must preserve the license and redistribution obligations of the dependencies it actually incorporates.
