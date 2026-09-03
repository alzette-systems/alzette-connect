# Third-party notices

Alzette Connect incorporates the open-source components listed below. Build-only
dependencies that are not distributed with the application are recorded in the
repository lockfiles.

## MIT-licensed components

- `github.com/adrg/xdg` — Copyright (c) 2014 Adrian-George Bostan
- `github.com/danieljoos/wincred` — Copyright (c) 2014 Daniel Joos
- `github.com/go-ole/go-ole` — Copyright © 2013-2017 Yasuhiro Matsumoto
- `github.com/mattn/go-colorable` — Copyright (c) 2016 Yasuhiro Matsumoto
- `github.com/mattn/go-isatty` — Copyright (c) Yasuhiro Matsumoto
- `github.com/pelletier/go-toml/v2` — Copyright (c) 2021-2023 Thomas Pelletier
- `github.com/wailsapp/wails/v3` and `@wailsio/runtime` — Copyright (c)
  2018-Present Lea Anthony
- Portions of `gopkg.in/yaml.v3` ported from libyaml — Copyright (c) 2006-2011
  Kirill Simonov

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Ollama launcher patterns

The Alzette Connect ChatGPT adapter adapts the reversible provider-profile,
model-catalogue, application discovery, and launch sequencing used by Ollama's
`cmd/launch/codex_app.go` and `cmd/launch/codex.go`, reviewed at commit
`f6c59d87038ae77f52d4adfbdc37363f8edd1ef3`.

Ollama is Copyright (c) Ollama and licensed under the MIT License printed in the
preceding section. Alzette Connect does not embed, launch, or require the Ollama
runtime. The adapted code retains Alzette's own OAuth identity, short-lived
human credential, loopback capability, policy, accounting, and cleanup
boundaries.

## `github.com/gofrs/flock` — BSD 3-Clause

Copyright (c) 2018-2025, The Gofrs
Copyright (c) 2015-2020, Tim Heckman
All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. Neither the name of gofrs nor the names of its contributors may be used to
   endorse or promote products derived from this software without specific
   prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## `golang.org/x/sys` — BSD 3-Clause

Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. Neither the name of Google LLC nor the names of its contributors may be used
   to endorse or promote products derived from this software without specific
   prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## `gopkg.in/yaml.v3` — Apache-2.0 portion

Files not covered by the libyaml MIT notice above are Copyright (c) 2011-2019
Canonical Ltd and licensed under Apache-2.0. The complete Apache-2.0 terms are
included in this distribution's `LICENSE` file.

## NSIS installer runtime

The Windows installer is generated with Nullsoft Scriptable Install System
(NSIS), Copyright (C) 1999-2026 Contributors. NSIS components are distributed
under OSI-approved licenses, primarily zlib/libpng; the LZMA compression module
uses the Common Public License 1.0 with the NSIS special exception. The complete
upstream terms are available in the
[NSIS license](https://nsis.sourceforge.io/Docs/AppendixI.html).
