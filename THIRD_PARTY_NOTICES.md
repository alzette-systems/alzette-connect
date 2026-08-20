# Third-party notices

## Ollama launcher patterns

The Alzette Connect ChatGPT adapter adapts the reversible provider-profile,
model-catalogue, application discovery, and launch sequencing used by Ollama's
`cmd/launch/codex_app.go` and `cmd/launch/codex.go`, reviewed at commit
`b7871fc0d1d82fe109536efa3e0e8e411c766c75`.

Ollama is Copyright (c) Ollama and licensed under the MIT License:

> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to
> deal in the Software without restriction, including without limitation the
> rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
> sell copies of the Software, and to permit persons to whom the Software is
> furnished to do so, subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in
> all copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
> SOFTWARE.

Alzette does not embed, launch, or require the Ollama runtime. The adapted
code retains Alzette's own OAuth identity, short-lived human credential,
loopback capability, policy, accounting, and cleanup boundaries.
