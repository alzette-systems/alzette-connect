const url = new URL(window.location.href);
const nativeHost = window.location.protocol === "wails:"
  || window.location.hostname === "wails.localhost"
  || url.searchParams.get("native") === "1";

if (nativeHost) {
  import("./native.js").catch(() => {
    window.AlzetteConnect?.setNativeMode(true);
    window.AlzetteConnect?.applyState({ phase: "failed", error_code: "service_unavailable", message: "The native connection is unavailable" });
  });
}
