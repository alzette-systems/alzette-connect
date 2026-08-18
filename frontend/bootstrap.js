const url = new URL(window.location.href);
const nativeHost = window.location.protocol === "wails:"
  || window.location.hostname === "wails.localhost"
  || url.searchParams.get("native") === "1";

if (nativeHost) {
  import("./native.js").catch(() => {
    window.AlzetteConnect?.setNativeMode(true);
    window.AlzetteConnect?.applyState({ state: "setup-attention", native: true });
  });
}
