(() => {
  "use strict";

  const STATES = new Set([
    "connected",
    "signed-out",
    "connecting",
    "offline",
    "no-models",
    "setup-attention",
    "access-ended",
    "protected-storage",
    "local-connection",
  ]);

  const COPY = {
    connected: {
      code: "Connection ready",
      title: "Connected",
      description: "Your private connection is ready for Jan and Goose.",
      runtime: "Connect starts when you sign in and stays ready in the background.",
    },
    "signed-out": {
      code: "Sign-in required",
      title: "Signed out",
      description: "Sign in on this device to reconnect your company models.",
      recovery: "Sign in",
      recoveryNote: "Your browser will open your company sign-in page.",
      runtime: "Application access managed by your company has not changed.",
    },
    connecting: {
      code: "Checking access",
      title: "Connecting…",
      description: "Checking your sign-in, company models, and local applications.",
      runtime: "This usually takes a few seconds. You can keep working while Connect checks.",
    },
    offline: {
      code: "Network unavailable",
      title: "You’re offline",
      description: "Connect can’t reach Alzette. Your application setup is still in place.",
      recovery: "Try again",
      recoveryNote: "We’ll check the connection without changing your application setup.",
      runtime: "Last known access remains listed below but cannot be used while offline.",
    },
    "no-models": {
      code: "Connection ready",
      title: "Connected",
      description: "Your connection is working, but no company models are available to you.",
      runtime: "Model access is assigned by your company owner through groups.",
    },
    "setup-attention": {
      code: "Application changed",
      title: "Setup needs attention",
      description: "Jan’s connection settings changed. Goose is still ready.",
      recovery: "Repair Jan setup",
      recoveryNote: "Connect will show the exact setting it restores before making a change.",
      runtime: "Your company sign-in and model access are still active.",
    },
    "access-ended": {
      code: "Company access ended",
      title: "Access ended",
      description: "Your company has ended access for this account on Alzette.",
      runtime: "This is not a connection error. Contact your company owner if you believe it is unexpected.",
    },
    "protected-storage": {
      code: "Sign-in storage locked",
      title: "Unlock this computer",
      description: "Connect can’t use the protected sign-in saved by this computer.",
      recovery: "Try again",
      recoveryNote: "Unlock Keychain, Credential Manager, or Secret Service, then retry.",
      runtime: "No sign-in credential was copied to a file or exposed to an application.",
    },
    "local-connection": {
      code: "Private connection unavailable",
      title: "Connection needs attention",
      description: "Connect could not start its private connection on this computer.",
      recovery: "Try again",
      recoveryNote: "Quit another copy of Connect if one is running, then retry.",
      runtime: "Jan and Goose were not reconfigured and no remote credential was exposed.",
    },
  };

  const DEFAULT_MODELS = [
    { name: "Luxembourg Chat", detail: "General work", id: "llama-3.3-70b", state: "available" },
    { name: "Document Reasoning", detail: "Long documents", id: "mistral-large", state: "available" },
  ];
  const DEFAULT_APPLICATIONS = [
    { id: "jan", status: "connected", version: "0.8.4" },
    { id: "goose", status: "connected", version: "1.46.0" },
  ];

  const state = {
    view: "onboarding",
    onboardingStep: "welcome",
    connection: "connected",
    company: "Northstar & Co.",
    checkedLabel: "Checked just now",
    checkedAt: "2026-08-18T11:42:00+02:00",
    models: DEFAULT_MODELS,
    applications: DEFAULT_APPLICATIONS,
    native: false,
  };

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));
  const delay = (milliseconds) => new Promise((resolve) => {
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    window.setTimeout(resolve, reduced ? Math.min(milliseconds, 80) : milliseconds);
  });

  const onboardingView = $("#onboarding-view");
  const statusView = $("#status-view");
  const demoState = $("#demo-state");
  const statusInstrument = $(".connection-instrument");
  const statusTitle = $("[data-status-title]");
  const statusDescription = $("[data-status-description]");
  const statusCode = $("[data-state-code]");
  const companyName = $("[data-company-name]");
  const checkedTime = $("[data-checked-time]");
  const runtimeNote = $("[data-runtime-note]");
  const modelList = $("[data-model-list]");
  const modelEmpty = $("[data-model-empty]");
  const recoveryPanel = $("[data-primary-recovery]");
  const recoveryAction = $("[data-recovery-action]");
  const recoveryNote = $("[data-recovery-note]");
  const announcer = $("[data-state-announcer]");
  const utilityButton = $("[data-utility-menu]");
  const utilityMenu = $(".utility-menu");
  const confirmationDialog = $("#confirmation-dialog");
  const toast = $(".toast");
  const setupCompany = $("[data-setup-company]");
  const setupSummary = $("[data-setup-summary]");
  const setupModelCount = $("[data-setup-model-count]");
  const completeCompany = $("[data-complete-company]");
  const completeModelCount = $("[data-complete-model-count]");
  const completeTime = $("[data-complete-time]");

  let toastTimer = 0;
  let dialogAction = "";
  let menuReturnFocus = null;

  function safeText(value, fallback, maxLength = 100) {
    if (typeof value !== "string") return fallback;
    const normalized = value.trim().replace(/[\u0000-\u001f\u007f]/g, "");
    return normalized ? normalized.slice(0, maxLength) : fallback;
  }

  function safeModels(models) {
    if (!Array.isArray(models)) return state.models;
    return models.slice(0, 3).map((model) => ({
      name: safeText(model && model.name, "Company model", 70),
      detail: safeText(model && model.detail, "Company model", 70),
      id: safeText(model && model.id, "model", 80),
      state: model && model.state === "not-ready" ? "not-ready" : "available",
    }));
  }

  function safeApplications(applications) {
    if (!Array.isArray(applications)) return state.applications;
    const allowed = new Set(["not_installed", "verification_required", "connected", "needs_attention"]);
    return applications
      .filter((application) => application && ["jan", "goose"].includes(application.id))
      .map((application) => ({
        id: application.id,
        status: allowed.has(application.status) ? application.status : "needs_attention",
        version: safeText(application.version, "", 30),
      }));
  }

  function emitAction(action, target = "") {
    const detail = { action, target, result: null };
    window.dispatchEvent(new CustomEvent("alzette:action", { detail }));
    return detail.result || Promise.resolve();
  }

  function showToast(message) {
    window.clearTimeout(toastTimer);
    toast.textContent = message;
    toast.hidden = false;
    toastTimer = window.setTimeout(() => {
      toast.hidden = true;
    }, 3600);
  }

  async function performAction(action, target, successMessage, prototypeMessage = successMessage) {
    try {
      await emitAction(action, target);
      if (successMessage) showToast(state.native ? successMessage : prototypeMessage);
      return true;
    } catch (error) {
      showToast(error instanceof Error ? error.message : "That action could not be completed.");
      return false;
    }
  }

  function setView(view, focus = true) {
    state.view = view === "status" ? "status" : "onboarding";
    const statusActive = state.view === "status";
    onboardingView.hidden = statusActive;
    statusView.hidden = !statusActive;
    $$('[data-demo-view]').forEach((button) => {
      button.setAttribute("aria-pressed", String(button.dataset.demoView === state.view));
    });
    $(".demo-dock__state").hidden = !statusActive;
    if (state.native) {
      void emitAction("window-mode", statusActive ? "compact" : "onboarding").catch(() => {});
    }
    if (focus) {
      const heading = statusActive
        ? statusTitle
        : $(`[data-step="${state.onboardingStep}"] h1`);
      heading?.focus({ preventScroll: true });
    }
  }

  function setOnboardingStep(step, focus = true) {
    const validSteps = ["welcome", "signin", "setup", "complete"];
    if (!validSteps.includes(step)) return;
    state.onboardingStep = step;
    const activeIndex = validSteps.indexOf(step);
    $$("[data-step]").forEach((panel) => {
      panel.hidden = panel.dataset.step !== step;
    });
    $$("[data-step-marker]").forEach((marker, index) => {
      const active = marker.dataset.stepMarker === step;
      if (active) marker.setAttribute("aria-current", "step");
      else marker.removeAttribute("aria-current");
      marker.dataset.complete = String(index < activeIndex);
      const mark = $(".setup-ledger__mark", marker);
      mark.textContent = index < activeIndex ? "✓" : String(index + 1);
    });
    if (focus) $(`[data-step="${step}"] h1`)?.focus({ preventScroll: true });
  }

  function makeModelRow(model, connectionState) {
    const item = document.createElement("li");
    const text = document.createElement("span");
    const name = document.createElement("strong");
    const detail = document.createElement("small");
    const id = document.createElement("span");
    const status = document.createElement("span");
    const statusIcon = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    const statusPath = document.createElementNS("http://www.w3.org/2000/svg", "path");

    name.textContent = model.name;
    detail.textContent = `${model.detail} · `;
    id.className = "mono";
    id.textContent = model.id;
    detail.append(id);
    text.append(name, detail);

    status.className = "row-state";
    statusIcon.setAttribute("viewBox", "0 0 16 16");
    statusIcon.setAttribute("aria-hidden", "true");
    const available = model.state === "available" && ["connected", "setup-attention"].includes(connectionState);
    const knownButOffline = model.state === "available" && connectionState === "offline";
    if (available) {
      status.classList.add("row-state--ready");
      statusPath.setAttribute("d", "m3 8 3 3 7-7");
      status.textContent = "Available";
    } else if (knownButOffline) {
      statusPath.setAttribute("d", "M3 8h10");
      status.textContent = "Offline";
    } else if (connectionState === "connecting") {
      statusPath.setAttribute("d", "M3 8h10");
      status.textContent = "Checking";
    } else {
      statusPath.setAttribute("d", "M3 8h10");
      status.textContent = "Not available yet";
    }
    statusIcon.append(statusPath);
    status.prepend(statusIcon);
    item.append(text, status);
    return item;
  }

  function setEmptyModels(title, description) {
    modelList.hidden = true;
    modelEmpty.hidden = false;
    $("strong", modelEmpty).textContent = title;
    $("p", modelEmpty).textContent = description;
  }

  function renderModels(connectionState) {
    modelList.replaceChildren();
    const hideForIdentity = connectionState === "signed-out" || connectionState === "access-ended";
    const noModels = connectionState === "no-models" || state.models.length === 0;

    if (connectionState === "signed-out") {
      setEmptyModels("Sign in to see company models", "Your company access will appear here after browser sign-in.");
      return;
    }
    if (connectionState === "access-ended") {
      setEmptyModels("Company models removed", `This device no longer has access to ${state.company} models.`);
      return;
    }
    if (noModels) {
      setEmptyModels("No company models available", "Your connection is working, but your company hasn’t assigned model access to you. Contact your company owner.");
      return;
    }
    modelList.hidden = hideForIdentity;
    modelEmpty.hidden = true;
    state.models.forEach((model) => modelList.append(makeModelRow(model, connectionState)));
  }

  function renderApps(connectionState) {
    ["jan", "goose"].forEach((app) => {
      const detail = $(`[data-app-detail="${app}"]`);
      const action = $(`[data-app-action="${app}"]`);
      const application = state.applications.find((value) => value.id === app)
        || { id: app, status: "not_installed", version: "" };
      action.disabled = false;
      action.textContent = "Open";

      if (connectionState === "signed-out") {
        detail.textContent = "Disconnected · sign in required";
        action.textContent = "Sign in";
      } else if (connectionState === "connecting") {
        detail.textContent = "Waiting for connection";
        action.textContent = "Waiting";
        action.disabled = true;
      } else if (connectionState === "offline") {
        detail.textContent = "Configured · unavailable offline";
        action.textContent = "Open";
      } else if (connectionState === "no-models") {
        detail.textContent = "Connected · no models assigned";
        action.textContent = "Open";
      } else if (connectionState === "setup-attention" && app === "jan") {
        detail.textContent = "Connection setting changed";
        action.textContent = "Repair";
      } else if (connectionState === "setup-attention") {
        detail.textContent = `Connected to ${state.models.length} models`;
      } else if (connectionState === "access-ended") {
        detail.textContent = "Company access removed";
        action.textContent = "Unavailable";
        action.disabled = true;
      } else if (application.status === "not_installed") {
        detail.textContent = "Not found in a supported location";
        action.textContent = "Unavailable";
        action.disabled = true;
      } else if (application.status === "verification_required") {
        detail.textContent = "Installed · setup verification required";
        action.textContent = "Set up";
      } else if (application.status === "needs_attention") {
        detail.textContent = "Setup needs attention";
        action.textContent = "Repair";
      } else {
        detail.textContent = `Connected to ${state.models.length} models`;
      }
    });
  }

  function renderSetupFacts() {
    setupCompany.textContent = state.company;
    completeCompany.textContent = state.company;
    const count = state.models.length;
    setupModelCount.textContent = `${count} company ${count === 1 ? "model" : "models"}`;
    completeModelCount.textContent = String(count);
    completeTime.textContent = "Just now";
    completeTime.dateTime = new Date().toISOString();
    const installed = state.applications.filter((application) => application.status !== "not_installed").length;
    setupSummary.textContent = installed === 2
      ? "Connect found Jan and Goose. It will verify their versions before changing either connection."
      : `Connect found ${installed} of 2 supported applications. Install and open each app once before setup.`;
    ["jan", "goose"].forEach((id) => {
      const application = state.applications.find((value) => value.id === id)
        || { status: "not_installed", version: "" };
      const detail = $(`[data-setup-app-detail="${id}"]`);
      const label = $(`[data-setup-app-status="${id}"]`);
      label.classList.toggle("status-label--ready", application.status !== "not_installed");
      if (application.status === "connected") {
        detail.textContent = `Version ${application.version} · verified`;
        label.textContent = "Connected";
      } else if (application.status === "verification_required") {
        detail.textContent = "Installed · version checked during setup";
        label.textContent = "Ready to verify";
      } else if (application.status === "needs_attention") {
        detail.textContent = "Installed · setup needs attention";
        label.textContent = "Needs attention";
      } else {
        detail.textContent = "Not found in a supported location";
        label.textContent = "Install app";
      }
    });
  }

  function renderRecovery(connectionState) {
    const copy = COPY[connectionState];
    const show = Boolean(copy.recovery);
    recoveryPanel.hidden = !show;
    if (!show) return;
    recoveryAction.textContent = copy.recovery;
    recoveryNote.textContent = copy.recoveryNote;
  }

  function applyState(payload = {}, announce = true) {
    const nextState = STATES.has(payload.state) ? payload.state : state.connection;
    const previousState = state.connection;
    state.connection = nextState;
    state.company = safeText(payload.company, state.company);
    state.checkedLabel = safeText(payload.checkedLabel, state.checkedLabel, 50);
    state.checkedAt = safeText(payload.checkedAt, state.checkedAt, 40);
    if (Object.hasOwn(payload, "models")) state.models = safeModels(payload.models);
    if (Object.hasOwn(payload, "applications")) state.applications = safeApplications(payload.applications);
    if (typeof payload.native === "boolean") setNativeMode(payload.native);

    const copy = COPY[nextState];
    statusInstrument.dataset.connectionState = nextState;
    statusCode.textContent = copy.code;
    statusTitle.textContent = copy.title;
    statusDescription.textContent = nextState === "access-ended"
      ? `${state.company} has ended access for this account on Alzette.`
      : copy.description;
    companyName.textContent = nextState === "signed-out" ? "No company selected" : state.company;
    checkedTime.textContent = nextState === "signed-out" ? "Not connected" : state.checkedLabel;
    checkedTime.dateTime = state.checkedAt;
    runtimeNote.textContent = copy.runtime;
    demoState.value = nextState;

    renderRecovery(nextState);
    renderModels(nextState);
    renderApps(nextState);
    renderSetupFacts();

    if (announce && previousState !== nextState) {
      announcer.textContent = `${copy.title}. ${copy.description}`;
    }
    return { state: nextState };
  }

  function setNativeMode(enabled = true) {
    state.native = Boolean(enabled);
    document.body.dataset.native = String(state.native);
  }

  function mapRuntimeSnapshot(snapshot) {
    if (!snapshot || typeof snapshot !== "object") return null;
    const phase = typeof snapshot.phase === "string" ? snapshot.phase : "starting";
    const contexts = Array.isArray(snapshot.contexts) ? snapshot.contexts : [];
    const context = contexts[0] && typeof contexts[0] === "object" ? contexts[0] : {};
    const phaseMap = {
      starting: "connecting",
      sign_in_required: "signed-out",
      signing_in: "connecting",
      ready: Array.isArray(context.models) && context.models.length === 0 ? "no-models" : "connected",
      no_access: "no-models",
      access_removed: "access-ended",
      offline: "offline",
      stopping: "offline",
      failed: "local-connection",
    };
    if (phase === "failed" && snapshot.error_code === "credential_store_unavailable") {
      phaseMap.failed = "protected-storage";
    } else if (phase === "failed" && snapshot.error_code === "local_proxy_unavailable") {
      phaseMap.failed = "local-connection";
    }
    const updated = typeof snapshot.updated_at === "string" ? new Date(snapshot.updated_at) : null;
    const modelIDs = Array.isArray(context.models) ? context.models : [];
    return {
      state: phaseMap[phase] || "connecting",
      company: safeText(context.organisation, "Your company"),
      checkedLabel: updated && !Number.isNaN(updated.valueOf())
        ? `Checked ${new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" }).format(updated)}`
        : state.checkedLabel,
      checkedAt: updated && !Number.isNaN(updated.valueOf()) ? updated.toISOString() : state.checkedAt,
      models: modelIDs.map((id) => ({ name: safeText(id, "Company model", 70), detail: "Company model", id, state: "available" })),
      applications: safeApplications(snapshot.applications),
      native: true,
    };
  }

  function closeUtilityMenu({ restoreFocus = true } = {}) {
    utilityMenu.hidden = true;
    utilityButton.setAttribute("aria-expanded", "false");
    if (restoreFocus && menuReturnFocus) menuReturnFocus.focus();
    menuReturnFocus = null;
  }

  function openUtilityMenu() {
    menuReturnFocus = document.activeElement;
    utilityMenu.hidden = false;
    utilityButton.setAttribute("aria-expanded", "true");
    $("[role='menuitem']", utilityMenu)?.focus();
  }

  function openConfirmation(action) {
    dialogAction = action;
    const title = $("#dialog-title");
    const description = $("#dialog-description");
    const confirm = $("[data-dialog-confirm]");
    if (action === "quit") {
      title.textContent = "Quit Alzette Connect?";
      description.textContent = "Jan and Goose will lose their private connection until you open Connect again.";
      confirm.textContent = "Quit Connect";
    } else {
      title.textContent = "Sign out on this device?";
      description.textContent = "Jan and Goose will disconnect from your company models. This won’t change access managed by your company.";
      confirm.textContent = "Sign out";
    }
    confirmationDialog.showModal();
  }

  $$('[data-demo-view]').forEach((button) => {
    button.addEventListener("click", () => setView(button.dataset.demoView));
  });

  demoState.addEventListener("change", () => {
    applyState({ state: demoState.value });
    setView("status", false);
  });

  $$('[data-next-step]').forEach((button) => {
    button.addEventListener("click", () => setOnboardingStep(button.dataset.nextStep));
  });
  $$('[data-previous-step]').forEach((button) => {
    button.addEventListener("click", () => setOnboardingStep(button.dataset.previousStep));
  });

  $("[data-browser-signin]").addEventListener("click", async (event) => {
    const button = event.currentTarget;
    button.disabled = true;
    $("[data-signin-wait]").hidden = false;
    try {
      await emitAction("begin-sign-in");
      if (!state.native) await delay(1100);
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Sign-in could not be completed.");
      $("[data-signin-wait]").hidden = true;
      button.disabled = false;
      return;
    }
    $("[data-signin-wait]").hidden = true;
    button.disabled = false;
    setOnboardingStep("setup");
  });

  $("[data-cancel-signin]").addEventListener("click", async () => {
    await performAction("cancel-sign-in", "", "Sign-in cancelled on this device.", "Sign-in would be cancelled.");
    $("[data-signin-wait]").hidden = true;
    $("[data-browser-signin]").disabled = false;
  });

  $("[data-configure-apps]").addEventListener("click", async (event) => {
    const button = event.currentTarget;
    button.disabled = true;
    button.textContent = "Verifying connection…";
    try {
      await emitAction("configure-apps", "jan,goose");
      if (!state.native) await delay(1300);
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Application setup could not be completed.");
      button.disabled = false;
      button.textContent = "Connect Jan and Goose";
      return;
    }
    button.disabled = false;
    button.textContent = "Connect Jan and Goose";
    setOnboardingStep("complete");
  });

  $("[data-finish-setup]").addEventListener("click", () => {
    applyState({ state: "connected" }, false);
    setView("status");
    emitAction("finish-onboarding");
  });

  utilityButton.setAttribute("aria-expanded", "false");
  utilityButton.addEventListener("click", () => {
    if (utilityMenu.hidden) openUtilityMenu();
    else closeUtilityMenu();
  });

  utilityMenu.addEventListener("keydown", (event) => {
    const items = $$("[role='menuitem']:not(:disabled)", utilityMenu);
    const index = items.indexOf(document.activeElement);
    if (event.key === "Escape") {
      event.preventDefault();
      closeUtilityMenu();
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      items[(index + 1) % items.length]?.focus();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      items[(index - 1 + items.length) % items.length]?.focus();
    } else if (event.key === "Home") {
      event.preventDefault();
      items[0]?.focus();
    } else if (event.key === "End") {
      event.preventDefault();
      items.at(-1)?.focus();
    }
  });

  document.addEventListener("pointerdown", (event) => {
    if (!utilityMenu.hidden && !utilityMenu.contains(event.target) && !utilityButton.contains(event.target)) {
      closeUtilityMenu({ restoreFocus: false });
    }
  });

  $$('[data-manage-portal]').forEach((button) => {
    button.addEventListener("click", async () => {
      closeUtilityMenu({ restoreFocus: false });
      await performAction("open-portal", "", "Opened the employee portal in your browser.", "The employee portal would open in your browser.");
    });
  });
  $("[data-manage-models]").addEventListener("click", async (event) => {
    event.preventDefault();
    await performAction("open-portal", "models", "Opened company models in your browser.", "Company models would open in the employee portal.");
  });
  $$('[data-open-help]').forEach((button) => {
    button.addEventListener("click", async () => {
      closeUtilityMenu({ restoreFocus: false });
      await performAction("open-help", "", "Opened Alzette Connect help.", "Help and privacy-safe diagnostics would open here.");
    });
  });
  $$('[data-open-app]').forEach((button) => {
    button.addEventListener("click", async () => {
      const appName = button.dataset.openApp === "jan" ? "Jan" : "Goose";
      await performAction("open-app", button.dataset.openApp, `Opening ${appName}…`, `${appName} would open now.`);
    });
  });
  $$('[data-app-action]').forEach((button) => {
    button.addEventListener("click", async () => {
      const app = button.dataset.appAction;
      if (state.connection === "signed-out") {
        await performAction("begin-sign-in", "", "Opened your company sign-in.", "Your company sign-in would open in a browser.");
      } else if (state.connection === "setup-attention" && app === "jan") {
        await performAction("repair-app", app, "Jan setup repaired.", "Jan repair would show the setting before it changes.");
      } else {
        const appName = app === "jan" ? "Jan" : "Goose";
        await performAction("open-app", app, `Opening ${appName}…`, `${appName} would open now.`);
      }
    });
  });

  recoveryAction.addEventListener("click", async () => {
    if (state.connection === "signed-out") {
      await performAction("begin-sign-in", "", "Opened your company sign-in.", "Your company sign-in would open in a browser.");
      return;
    }
    if (state.connection === "setup-attention") {
      await performAction("repair-app", "jan", "Jan setup repaired.", "Jan repair would show the setting before it changes.");
      return;
    }
    if (!(await performAction("retry-connection", "", "", ""))) return;
    applyState({ state: "connecting" });
    await delay(1400);
    applyState({ state: "connected", checkedLabel: "Checked just now" });
  });

  $("[data-sign-out]").addEventListener("click", () => {
    closeUtilityMenu({ restoreFocus: false });
    openConfirmation("sign-out");
  });
  $("[data-quit]").addEventListener("click", () => {
    closeUtilityMenu({ restoreFocus: false });
    openConfirmation("quit");
  });
  confirmationDialog.addEventListener("close", async () => {
    if (confirmationDialog.returnValue !== "confirm") return;
    if (dialogAction === "quit") {
      await performAction("quit", "", "", "The native app would now quit and disconnect local applications.");
    } else {
      if (await performAction("sign-out", "", "Signed out on this device.")) {
        applyState({ state: "signed-out" });
      }
    }
  });

  window.addEventListener("alzette:state", (event) => {
    if (event instanceof CustomEvent && event.detail && typeof event.detail === "object") {
      applyState(event.detail);
    }
  });

  const params = new URLSearchParams(window.location.search);
  const initialView = params.get("view") === "status" ? "status" : "onboarding";
  const initialState = STATES.has(params.get("state")) ? params.get("state") : "connected";
  if (params.get("native") === "1") setNativeMode(true);
  setOnboardingStep("welcome", false);
  applyState({ state: initialState }, false);
  setView(initialView, false);
  window.AlzetteConnect = Object.freeze({
    applyState,
    showView: setView,
    setNativeMode,
    mapRuntimeSnapshot,
  });
})();
