import { Events } from "@wailsio/runtime";
import {
  BeginSignIn,
  CancelLaunch,
  CancelSignIn,
  CheckForUpdates,
  CurrentState,
  Disconnect,
  HideToTray,
  LaunchApplication,
  OpenPortal,
  Quit,
  RetryCleanup,
  SelectContext,
  SignOut,
  InstallUpdate,
  SetWindowMode,
} from "./bindings/github.com/ticruz38/alzette-connect/desktopservice.js";

function present(snapshot) {
  const mapped = window.AlzetteConnect?.mapRuntimeSnapshot(snapshot);
  if (!mapped) return;
  window.AlzetteConnect.setNativeMode(true);
  window.AlzetteConnect.applyState(mapped);
}

function runAction(detail) {
  const { action, target = "" } = detail;
  switch (action) {
    case "begin-sign-in":
      interactiveSignIn = true;
      return BeginSignIn();
    case "retry-connection":
      return BeginSignIn();
    case "cancel-sign-in":
      interactiveSignIn = false;
      CancelSignIn();
      return Promise.resolve();
    case "select-context":
      return SelectContext(target);
    case "window-mode":
      return SetWindowMode(target);
    case "open-portal":
      return OpenPortal(target);
    case "sign-out":
      return SignOut();
    case "quit":
      Quit();
      return Promise.resolve();
    case "launch-application":
      return LaunchApplication(target);
    case "cancel-launch":
      CancelLaunch();
      return Promise.resolve();
    case "disconnect":
      return Disconnect();
    case "retry-cleanup":
      return RetryCleanup();
    case "hide-to-tray":
      HideToTray();
      return Promise.resolve();
    case "check-update":
      return CheckForUpdates();
    case "install-update":
      return InstallUpdate();
    default:
      return Promise.reject(new Error("Alzette Connect did not recognise that action."));
  }
}

let interactiveSignIn = false;

window.addEventListener("alzette:action", (event) => {
  if (!(event instanceof CustomEvent) || !event.detail) return;
  event.detail.result = runAction(event.detail);
});

Events.On("connect:state", (event) => {
  present(event.data);
});

async function initialiseNativeBridge() {
  window.AlzetteConnect?.setNativeMode(true);
  try {
    present(await CurrentState());
  } catch {
    window.AlzetteConnect?.applyState({ phase: "failed", error_code: "service_unavailable", message: "Alzette Connect is still starting" });
  }
}

if (document.readyState === "loading") {
  window.addEventListener("DOMContentLoaded", initialiseNativeBridge, { once: true });
} else {
  initialiseNativeBridge();
}
