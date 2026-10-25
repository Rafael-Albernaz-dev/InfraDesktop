# InfraDesktop icon assets

These application icons reproduce the project's own geometric network mark: three rectangular nodes, an orthogonal branch and a turquoise rounded square. They were authored locally for the authorized BF01 delivery; no external illustration, font, icon package, logo catalog, image model or downloaded bitmap was used.

The source mark supplied with the project uses a 36×36 rounded square, a centered 26×26 SVG with viewBox 0 0 32 32, and a 1.6-unit stroke. The application icon retains the light-reference turquoise #0e7490 and white foreground as a fixed identity suitable for light and dark launchers. PNG corners are transparent. The application icon is not a replacement for theme-specific UI marks.

The editable source is `infradesktop.svg`. Reproduce all sizes from the shared geometry in `scripts/generate-infradesktop-icon.py`:

    python3 scripts/generate-infradesktop-icon.py
    python3 scripts/generate-infradesktop-icon.py --check

The script uses Python's standard library and Pillow. Pillow is a build tool only (locally verified version 10.2.0, HPND license); its code is not bundled in the PNGs or SVG. Runtime does not depend on Python, Pillow, or the private reference documentation.

Licensing: these are first-party InfraDesktop project assets and follow the project's licensing policy. No separate license grant or project license change is made by this delivery. The checkout did not contain a top-level LICENSE file at preparation time. There are no third-party artwork attribution requirements introduced by these assets. This note records provenance and does not assign ownership or select a license on the user's behalf.
