import { Events } from "@wailsio/runtime";
import {
  BeginSignIn,
  CancelSignIn,
  CheckForUpdates,
  ConfigureApps,
  CurrentState,
  OpenApp,
  OpenPortal,
  Quit,
  SignOut,
  InstallUpdate,
  SetWindowMode,
} from "./bindings/github.com/ticruz38/alzette-connect/desktopservice.js";

function present(snapshot, chooseView = false) {
  const mapped = window.AlzetteConnect?.mapRuntimeSnapshot(snapshot);
  if (!mapped) return;
  window.AlzetteConnect.setNativeMode(true);
  window.AlzetteConnect.applyState(mapped);
  if (chooseView) {
    window.AlzetteConnect.showView(mapped.state === "signed-out" ? "onboarding" : "status", false);
  }
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
    case "window-mode":
      return SetWindowMode(target);
    case "open-portal":
      return OpenPortal(target);
    case "sign-out":
      return SignOut();
    case "quit":
      Quit();
      return Promise.resolve();
    case "configure-apps":
      return ConfigureApps(target);
    case "finish-onboarding":
      return Promise.resolve();
    case "open-app":
      return OpenApp(target);
    case "repair-app":
      return ConfigureApps(target);
    case "open-help":
      return Promise.reject(new Error("This action is not available in this build yet."));
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
  const mapped = window.AlzetteConnect?.mapRuntimeSnapshot(event.data);
  const chooseView = !interactiveSignIn && mapped?.state !== "signed-out";
  present(event.data, chooseView);
});

async function initialiseNativeBridge() {
  window.AlzetteConnect?.setNativeMode(true);
  try {
    present(await CurrentState(), true);
  } catch {
    window.AlzetteConnect?.applyState({ state: "setup-attention", native: true });
  }
}

if (document.readyState === "loading") {
  window.addEventListener("DOMContentLoaded", initialiseNativeBridge, { once: true });
} else {
  initialiseNativeBridge();
}
