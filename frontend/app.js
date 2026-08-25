(() => {
  "use strict";

  const APPLICATION_NAMES = {
    pi: "Pi",
    jan: "Jan Desktop",
    goose: "Goose Desktop",
    chatgpt: "ChatGPT",
  };
  const STATUS_COPY = {
    ready: { label: "Ready", className: "state--ready", icon: "✓", selectable: true },
    verification_required: { label: "Verify at launch", className: "state--choice", icon: "1", selectable: true },
    needs_attention: { label: "Setup needs attention", className: "state--warning", icon: "!", selectable: true },
    not_installed: { label: "Not installed", className: "state--choice", icon: "–", selectable: false },
    update_required: { label: "Update required", className: "state--warning", icon: "!", selectable: false },
    protocol_unavailable: { label: "Protocol unavailable", className: "state--unavailable", icon: "×", selectable: false },
    blocked_by_company: { label: "Blocked by company", className: "state--warning", icon: "!", selectable: false },
    not_supported: { label: "Not yet supported", className: "state--unavailable", icon: "×", selectable: false },
  };
  const UPDATE_STATES = new Set(["idle", "checking", "current", "available", "downloading", "installing", "installer_opened", "error"]);

  const state = {
    native: false,
    snapshot: normalizeSnapshot({ phase: "sign_in_required", launch: { phase: "idle" } }),
    selectedApp: "",
    pendingLaunch: false,
    signingIn: false,
    launchError: "",
    recoveryDismissed: false,
  };

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));
  const screens = $$("[data-screen]");
  const authOnly = $$("[data-auth-only]");
  const menuButton = $("[data-menu-button]");
  const accountMenu = $("[data-account-menu]");
  const catalogueToggle = $("[data-catalogue-toggle]");
  const modelDrawer = $("[data-model-drawer]");
  const applicationList = $("[data-application-list]");
  const launchPlane = $("[data-launch-plane]");
  const launchButton = $("[data-launch]");
  const liveRegion = $("[data-live-region]");
  const toast = $("[data-toast]");
  const sessionDialog = $("[data-session-dialog]");
  const diagnosticsDialog = $("[data-diagnostics-dialog]");
  let toastTimer = 0;

  function safeText(value, fallback = "", maxLength = 120) {
    if (typeof value !== "string") return fallback;
    const normalized = value.trim().replace(/[\u0000-\u001f\u007f]/g, "");
    return normalized ? normalized.slice(0, maxLength) : fallback;
  }

  function normalizeSnapshot(input) {
    const source = input && typeof input === "object" ? input : {};
    const contexts = Array.isArray(source.contexts) ? source.contexts.slice(0, 24).map((context) => ({
      id: safeText(context?.id, "", 160),
      organisation: safeText(context?.organisation, "Company", 100),
      project: safeText(context?.project, "", 100),
      environment: safeText(context?.environment, "", 100),
      models: Array.isArray(context?.models)
        ? [...new Set(context.models.map((model) => safeText(model, "", 128)).filter(Boolean))].slice(0, 64)
        : [],
    })).filter((context) => context.id) : [];
    const applications = Array.isArray(source.applications) ? source.applications.slice(0, 12).map((application) => ({
      id: safeText(application?.id, "", 40),
      name: safeText(application?.name, APPLICATION_NAMES[application?.id] || "Application", 70),
      status: STATUS_COPY[application?.status] ? application.status : "not_supported",
      version: safeText(application?.version, "", 30),
      detail: safeText(application?.detail, "", 100),
      deliveryMode: safeText(application?.delivery_mode ?? application?.deliveryMode, "catalogue", 40),
      modelCount: Number.isInteger(application?.model_count) ? Math.max(0, application.model_count)
        : Number.isInteger(application?.modelCount) ? Math.max(0, application.modelCount) : 0,
      installed: application?.installed === true,
      configured: application?.configured === true,
    })).filter((application) => application.id) : [];
    const launchSource = source.launch && typeof source.launch === "object" ? source.launch : {};
    const updateSource = source.update && typeof source.update === "object" ? source.update : {};
    return {
      phase: safeText(source.phase, "sign_in_required", 40),
      message: safeText(source.message, "", 180),
      errorCode: safeText(source.error_code ?? source.errorCode, "", 80),
      selectedContextID: safeText(source.selected_context_id ?? source.selectedContextID, "", 160),
      contexts,
      applications,
      launch: {
        phase: safeText(launchSource.phase, "idle", 30) || "idle",
        applicationID: safeText(launchSource.application_id ?? launchSource.applicationID, "", 40),
        application: safeText(launchSource.application, "", 70),
        message: safeText(launchSource.message, "", 180),
        startedAt: safeText(launchSource.started_at ?? launchSource.startedAt, "", 50),
        modelCount: Number.isInteger(launchSource.model_count) ? Math.max(0, launchSource.model_count)
          : Number.isInteger(launchSource.modelCount) ? Math.max(0, launchSource.modelCount) : 0,
        cleanupPending: launchSource.cleanup_pending === true || launchSource.cleanupPending === true,
        localClosed: launchSource.local_closed === true || launchSource.localClosed === true,
        grantStatus: safeText(launchSource.grant_status ?? launchSource.grantStatus, "", 30),
        profileStatus: safeText(launchSource.profile_status ?? launchSource.profileStatus, "", 30),
      },
      update: {
        state: UPDATE_STATES.has(updateSource.state) ? updateSource.state : "idle",
        currentVersion: safeText(updateSource.current_version ?? updateSource.currentVersion, "", 40),
        availableVersion: safeText(updateSource.available_version ?? updateSource.availableVersion, "", 40),
        message: safeText(updateSource.message, "", 180),
      },
      platform: safeText(source.platform, "", 20),
      trayAvailable: source.tray_available === true || source.trayAvailable === true,
      updatedAt: safeText(source.updated_at ?? source.updatedAt, "", 50),
    };
  }

  function selectedContext() {
    const { contexts, selectedContextID } = state.snapshot;
    return contexts.find((context) => context.id === selectedContextID)
      || (contexts.length === 1 ? contexts[0] : null);
  }

  function emitAction(action, target = "") {
    if (!state.native) return Promise.reject(new Error("This action is available in the installed desktop app."));
    const detail = { action, target, result: null };
    window.dispatchEvent(new CustomEvent("alzette:action", { detail }));
    return detail.result || Promise.reject(new Error("Alzette Connect is still starting."));
  }

  function showToast(message) {
    window.clearTimeout(toastTimer);
    toast.textContent = safeText(message, "That action could not be completed.", 200);
    toast.hidden = false;
    toastTimer = window.setTimeout(() => { toast.hidden = true; }, 3800);
  }

  function screenForSnapshot() {
    const launchPhase = state.snapshot.launch.phase;
    if (state.pendingLaunch || launchPhase === "preparing" || launchPhase === "disconnecting") return "preparing";
    if (launchPhase === "running") return "running";
    if ((launchPhase === "recovery" || state.snapshot.launch.cleanupPending) && !state.recoveryDismissed) return "recovery";
    if (state.snapshot.phase === "no_access" && state.snapshot.errorCode === "context_selection_required") return "contexts";
    if (state.snapshot.phase === "ready" || state.snapshot.phase === "no_access"
      || (["offline", "failed"].includes(state.snapshot.phase) && state.snapshot.contexts.length > 0)) return "launcher";
    return "signed-out";
  }

  function showScreen(name, focus = false) {
    screens.forEach((screen) => { screen.hidden = screen.dataset.screen !== name; });
    authOnly.forEach((node) => { node.hidden = name === "signed-out" || name === "contexts"; });
    if (name !== "launcher") closeCatalogue();
    if (state.native) void emitAction("window-mode", name === "signed-out" ? "signed-out" : "launcher").catch(() => {});
    if (focus) {
      window.requestAnimationFrame(() => $(`[data-screen="${name}"] h1`)?.focus({ preventScroll: true }));
    }
  }

  function renderHeader(context, screen) {
    $("[data-company-name]").textContent = context?.organisation || "";
    $("[data-workspace-name]").textContent = context ? [context.project, context.environment].filter(Boolean).join(" / ") : "";
    $("[data-menu-company]").textContent = context?.organisation || "Alzette Connect";
    $("[data-menu-context]").textContent = context
      ? [context.project, context.environment].filter(Boolean).join(" / ")
      : screen === "signed-out" ? "Signed out on this device" : "Choose a company workspace";
  }

  function renderSignIn() {
    const button = $("[data-sign-in]");
    const cancel = $("[data-cancel-sign-in]");
    const detail = $("[data-sign-in-detail]");
    const busy = state.signingIn || state.snapshot.phase === "signing_in";
    const accessRemoved = state.snapshot.phase === "access_removed";
    $("[data-sign-in-label]").textContent = accessRemoved ? "COMPANY ACCESS" : "EMPLOYEE CONNECTION";
    $("[data-sign-in-title]").textContent = accessRemoved ? "Your company access has ended." : "Launch your approved AI tools.";
    $("[data-sign-in-copy]").textContent = accessRemoved
      ? "This device can no longer load company models. Contact your company owner if this is unexpected."
      : "Sign in to load the models assigned by your company. Connect keeps remote credentials out of desktop applications.";
    $("[data-sign-in-facts]").hidden = accessRemoved;
    $("[data-portal-models]").hidden = accessRemoved;
    button.disabled = busy;
    button.textContent = accessRemoved ? "Sign out on this device"
      : busy ? "Continue in your browser…" : "Sign in with Alzette";
    cancel.hidden = !busy;
    if (state.snapshot.phase === "access_removed") {
      detail.textContent = "Your company access has ended. Contact your company owner if this is unexpected.";
    } else if (state.snapshot.errorCode === "credential_store_unavailable") {
      detail.textContent = "Unlock this computer’s protected credential store, then try again.";
    } else if (state.snapshot.errorCode === "service_unavailable") {
      detail.textContent = "Alzette could not be reached. Check your connection and try again.";
    } else if (state.snapshot.errorCode === "sign_in_cancelled") {
      detail.textContent = "Sign-in was not completed. You can try again.";
    } else {
      detail.textContent = "Authentication opens in your default browser.";
    }
  }

  function renderContexts() {
    const list = $("[data-context-list]");
    list.replaceChildren();
    state.snapshot.contexts.forEach((context) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "context-row";
      const copy = document.createElement("span");
      const name = document.createElement("strong");
      const detail = document.createElement("small");
      const action = document.createElement("em");
      name.textContent = context.organisation;
      detail.textContent = [context.project, context.environment, `${context.models.length} model${context.models.length === 1 ? "" : "s"}`].filter(Boolean).join(" · ");
      action.textContent = "Choose";
      copy.append(name, detail);
      button.append(copy, action);
      button.addEventListener("click", async () => {
        button.disabled = true;
        try { await emitAction("select-context", context.id); }
        catch (error) { showToast(error instanceof Error ? error.message : "That workspace is no longer available."); button.disabled = false; }
      });
      list.append(button);
    });
  }

  function renderModels(context) {
    const models = context?.models || [];
    const authorityCurrent = state.snapshot.phase === "ready" || state.snapshot.phase === "no_access";
    $("[data-catalogue-count]").textContent = models.length
      ? `${models.length} model${models.length === 1 ? "" : "s"} available through Alzette`
      : "No company models available";
    $("[data-catalogue-freshness]").textContent = !authorityCurrent
      ? "Last known company access · reconnect required"
      : models.length ? "Current company access · synchronized now" : "Your company owner manages access through groups";
    const list = $("[data-model-list]");
    const empty = $("[data-model-empty]");
    list.replaceChildren();
    empty.hidden = models.length !== 0;
    models.forEach((alias) => {
      const item = document.createElement("li");
      const copy = document.createElement("span");
      const name = document.createElement("b");
      const detail = document.createElement("small");
      const status = document.createElement("em");
      name.textContent = alias;
      detail.textContent = "Company model alias";
      status.textContent = "Available";
      copy.append(name, detail);
      item.append(copy, status);
      list.append(item);
    });
  }

  function applicationStatus(application, context) {
    if (state.snapshot.phase !== "ready" && state.snapshot.phase !== "no_access") {
      return { label: "Reconnect required", className: "state--warning", icon: "!", selectable: false };
    }
    if (!context || context.models.length === 0) {
      return { label: "No models assigned", className: "state--choice", icon: "–", selectable: false };
    }
    return STATUS_COPY[application.status] || STATUS_COPY.not_supported;
  }

  function makeApplicationRow(application, context, running = false) {
    const status = applicationStatus(application, context);
    const row = document.createElement(running ? "div" : "button");
    if (!running) row.type = "button";
    row.className = `application-row${running ? " is-disabled" : ""}`;
    row.dataset.app = application.id;
    row.dataset.mode = application.deliveryMode;
    if (!running) {
      row.setAttribute("aria-disabled", String(!status.selectable));
      if (status.selectable) row.setAttribute("aria-pressed", String(application.id === state.selectedApp));
      if (application.id === state.selectedApp) row.classList.add("is-selected");
    }

    const icon = document.createElement("span");
    icon.className = `app-icon${application.id === "pi" ? " app-icon--pi" : ""}`;
    icon.setAttribute("aria-hidden", "true");
    icon.textContent = application.id === "pi" ? "π" : (application.name[0] || "A").toUpperCase();

    const name = document.createElement("span");
    name.className = "application-row__name";
    const strong = document.createElement("strong");
    const small = document.createElement("small");
    strong.textContent = application.name;
    const version = application.version ? ` · ${application.version}` : "";
    small.textContent = application.installed ? `Installed${version}` : application.detail || "Not found in a supported location";
    name.append(strong, small);

    const models = document.createElement("span");
    models.className = "application-row__models";
    const modelsStrong = document.createElement("strong");
    const modelsSmall = document.createElement("small");
    modelsStrong.textContent = application.modelCount > 0 ? `${application.modelCount} model${application.modelCount === 1 ? "" : "s"}` : "—";
    modelsSmall.textContent = application.deliveryMode === "catalogue" ? "Full catalogue"
      : application.deliveryMode === "primary_plus_catalogue" ? "Primary + catalogue"
      : application.deliveryMode === "single" ? "Single model at launch" : "Adapter-specific";
    models.append(modelsStrong, modelsSmall);

    const stateNode = document.createElement("span");
    stateNode.className = `state ${status.className}`;
    const stateIcon = document.createElement("i");
    stateIcon.setAttribute("aria-hidden", "true");
    stateIcon.textContent = status.icon;
    stateNode.append(stateIcon, document.createTextNode(` ${status.label}`));
    row.append(icon, name, models, stateNode);

    if (!running && status.selectable) {
      row.addEventListener("click", () => selectApplication(application.id));
      row.addEventListener("dblclick", () => { selectApplication(application.id); void launchSelected(); });
      row.addEventListener("keydown", (event) => {
        if (event.key === "Enter") { event.preventDefault(); selectApplication(application.id); void launchSelected(); }
      });
    }
    return row;
  }

  function renderApplications(context) {
    applicationList.replaceChildren();
    const available = state.snapshot.applications;
    if (!available.some((application) => application.id === state.selectedApp && applicationStatus(application, context).selectable)) {
      state.selectedApp = available.find((application) => applicationStatus(application, context).selectable)?.id || "";
    }
    available.forEach((application) => {
      const item = document.createElement("li");
      item.append(makeApplicationRow(application, context));
      applicationList.append(item);
    });
    const selected = available.find((application) => application.id === state.selectedApp);
    if (!selected) {
      launchPlane.hidden = true;
      return;
    }
    const count = selected.modelCount;
    $("[data-launch-title]").textContent = `${selected.status === "ready" ? "Launch" : "Verify and launch"} ${selected.name} through Alzette`;
    const catalogueHint = selected.deliveryMode === "catalogue"
      ? `Choose a model inside ${selected.name}.`
      : selected.deliveryMode === "primary_plus_catalogue"
        ? "It opens on the first listed company model and keeps the complete catalogue available in its model picker."
        : "";
    $("[data-launch-detail]").textContent = `${selected.name} will receive ${count === 1 ? "the assigned model" : `all ${count} compatible models`}. ${catalogueHint}`;
    launchButton.textContent = `${selected.status === "ready" ? "Launch" : "Verify and launch"} ${selected.name}`;
    launchPlane.hidden = false;
  }

  function selectApplication(id) {
    state.selectedApp = id;
    renderApplications(selectedContext());
  }

  async function launchSelected() {
    if (!state.selectedApp || state.pendingLaunch) return;
    state.pendingLaunch = true;
    state.launchError = "";
    render();
    try {
      await emitAction("launch-application", state.selectedApp);
    } catch (error) {
      state.pendingLaunch = false;
      state.launchError = error instanceof Error ? error.message : "Application launch could not be completed.";
      render();
    }
  }

  function renderLauncher(context) {
    renderModels(context);
    renderApplications(context);
    const degraded = state.snapshot.phase === "offline" || state.snapshot.phase === "failed";
    $("[data-authority-warning]").hidden = !degraded;
    $("[data-authority-detail]").textContent = state.snapshot.message || "Reconnect before launching an application.";
    $("[data-cleanup-banner]").hidden = !state.snapshot.launch.cleanupPending;
    const error = $("[data-launch-error]");
    error.hidden = !state.launchError;
    $("[data-launch-error-detail]").textContent = state.launchError || "Review the application status and try again.";
  }

  function renderPreparing() {
    const launch = state.snapshot.launch;
    const application = launch.application || APPLICATION_NAMES[state.selectedApp] || "application";
    const count = launch.modelCount || selectedContext()?.models.length || 0;
    $("[data-preparing-label]").textContent = `LAUNCHING ${application.toUpperCase()}`;
    $("[data-preparing-models]").textContent = `${count} model${count === 1 ? "" : "s"} available through Alzette`;
  }

  function renderRunning(context) {
    const launch = state.snapshot.launch;
    const application = state.snapshot.applications.find((item) => item.id === launch.applicationID)
      || { id: launch.applicationID, name: launch.application || "Application", status: "ready", installed: true, modelCount: launch.modelCount, deliveryMode: "catalogue" };
    $("[data-running-models]").textContent = `${launch.modelCount} model${launch.modelCount === 1 ? "" : "s"} available through Alzette`;
    $("[data-running-catalogue]").textContent = `Current company access · ${application.name} is using this catalogue`;
    $("[data-running-title]").textContent = `${application.name} is running`;
    $("[data-session-title]").textContent = `${application.name} is connected through Alzette`;
    $("[data-supervision-copy]").textContent = state.snapshot.trayAvailable
      ? "Connect keeps supervision active in the tray."
      : "Keep this window open while Connect supervises the session.";
    $("[data-hide-tray]").textContent = state.snapshot.trayAvailable ? "Hide to tray" : "Hide window";
    $("[data-hide-tray]").hidden = !state.snapshot.trayAvailable;
    const list = $("[data-running-list]");
    list.replaceChildren();
    const activeRow = makeApplicationRow(application, context, true);
    activeRow.classList.remove("is-disabled");
    activeRow.classList.add("is-running");
    const stateNode = $(".state", activeRow);
    stateNode.className = "state state--running";
    stateNode.replaceChildren(Object.assign(document.createElement("i"), { textContent: "" }), document.createTextNode(" Running"));
    const activeItem = document.createElement("li");
    activeItem.append(activeRow);
    list.append(activeItem);
    state.snapshot.applications.filter((item) => item.id !== application.id).forEach((item) => {
      const listItem = document.createElement("li");
      listItem.append(makeApplicationRow(item, context, true));
      list.append(listItem);
    });
  }

  function renderDiagnostics(context) {
    $("[data-version-label]").textContent = state.snapshot.update.currentVersion || "Build version unavailable";
    $("[data-diagnostic-connection]").textContent = state.snapshot.phase === "ready" ? "Signed in; no inference session until launch" : state.snapshot.message || state.snapshot.phase;
    $("[data-diagnostic-workspace]").textContent = context ? `${context.organisation} · ${[context.project, context.environment].filter(Boolean).join(" / ")}` : "No workspace selected";
    const ready = state.snapshot.applications.filter((application) => application.status === "ready").length;
    const installed = state.snapshot.applications.filter((application) => application.installed).length;
    $("[data-diagnostic-apps]").textContent = `${installed} detected · ${ready} qualified in this session`;
    const update = state.snapshot.update;
    $("[data-update-title]").textContent = update.state === "available" ? `Connect ${update.availableVersion} is available`
      : update.state === "current" ? "Connect is current"
      : update.state === "checking" ? "Checking for updates…"
      : update.state === "error" ? "Update check needs attention"
      : "Updates";
    $("[data-update-message]").textContent = update.message || "Check the internal release channel when you are ready.";
    $("[data-check-update]").disabled = ["checking", "downloading", "installing"].includes(update.state);
    $("[data-install-update]").hidden = update.state !== "available";
  }

  function render() {
    const context = selectedContext();
    const screen = screenForSnapshot();
    renderHeader(context, screen);
    renderSignIn();
    renderContexts();
    if (screen === "launcher") renderLauncher(context);
    if (screen === "preparing") renderPreparing();
    if (screen === "running") renderRunning(context);
    if (screen === "recovery") {
      const profileNeedsReview = state.snapshot.launch.profileStatus !== "restored";
      const grantNeedsRetry = state.snapshot.launch.grantStatus !== "confirmed";
      const applicationName = state.snapshot.launch.application || "The application";
      $("[data-recovery-strip-title]").textContent = profileNeedsReview ? `${applicationName} saved newer local settings.` : "Remote revocation needs another check.";
      $("[data-recovery-strip-copy]").textContent = profileNeedsReview
        ? "The private connection is closed; Connect preserved ChatGPT's newer settings."
        : "The local connection and application profile are already safe.";
      $("[data-recovery-title]").textContent = profileNeedsReview ? `Finish cleaning up ${applicationName}` : "Finish disconnecting";
      $("[data-recovery-message]").textContent = state.snapshot.launch.message || "Connect could not confirm every cleanup step.";
      $("[data-recovery-local]").textContent = state.snapshot.launch.localClosed ? "Closed" : "Confirmation unavailable";
      $("[data-recovery-grant]").textContent = state.snapshot.launch.grantStatus === "confirmed" ? "Revocation confirmed" : "Revocation not confirmed";
      $("[data-recovery-profile]").textContent = profileNeedsReview ? "Newer settings preserved" : "Restored";
      $("[data-retry-cleanup]").textContent = profileNeedsReview && grantNeedsRetry ? "Retry safe cleanup" : profileNeedsReview ? "Restore previous profile" : "Check revocation again";
    }
    renderDiagnostics(context);
    showScreen(screen);
    liveRegion.textContent = screen === "running" ? `${state.snapshot.launch.application} is running through Alzette.`
      : screen === "preparing" ? `Preparing ${state.snapshot.launch.application || APPLICATION_NAMES[state.selectedApp] || "the application"}.`
      : state.snapshot.message;
  }

  function closeCatalogue() {
    modelDrawer.hidden = true;
    catalogueToggle.setAttribute("aria-expanded", "false");
  }

  function setNativeMode(value) {
    state.native = value === true;
    document.body.dataset.native = String(state.native);
  }

  function applyState(snapshot) {
    const previousScreen = screenForSnapshot();
    const hadCleanup = state.snapshot.launch.cleanupPending;
    state.snapshot = normalizeSnapshot(snapshot);
    if (!state.snapshot.launch.cleanupPending) state.recoveryDismissed = false;
    else if (!hadCleanup) state.recoveryDismissed = false;
    if (state.snapshot.launch.phase !== "preparing") state.pendingLaunch = false;
    if (state.snapshot.launch.phase === "running") state.launchError = "";
    render();
    const nextScreen = screenForSnapshot();
    if (nextScreen !== previousScreen) showScreen(nextScreen, true);
  }

  $("[data-sign-in]").addEventListener("click", async () => {
    if (state.snapshot.phase === "access_removed") {
      try { await emitAction("sign-out"); } catch (error) { showToast(error instanceof Error ? error.message : "Sign-out could not be completed."); }
      return;
    }
    state.signingIn = true;
    renderSignIn();
    try { await emitAction("begin-sign-in"); }
    catch (error) {
      if (state.snapshot.errorCode !== "context_selection_required") showToast(error instanceof Error ? error.message : "Sign-in could not be completed.");
    } finally { state.signingIn = false; render(); }
  });
  $("[data-cancel-sign-in]").addEventListener("click", () => { void emitAction("cancel-sign-in").catch(() => {}); });
  $("[data-context-sign-out]").addEventListener("click", () => { void emitAction("sign-out").catch((error) => showToast(error.message)); });
  launchButton.addEventListener("click", () => { void launchSelected(); });
  $("[data-cancel-launch]").addEventListener("click", () => { void emitAction("cancel-launch").catch(() => {}); });
  $("[data-disconnect]").addEventListener("click", async () => {
    try { await emitAction("disconnect"); }
    catch (error) { showToast(error instanceof Error ? error.message : "Disconnect could not be confirmed."); }
  });
  $("[data-hide-tray]").addEventListener("click", () => { void emitAction("hide-to-tray").catch(() => {}); });
  $("[data-back-launcher]").addEventListener("click", () => {
    state.recoveryDismissed = true;
    render();
  });
  $("[data-review-cleanup]").addEventListener("click", () => {
    state.recoveryDismissed = false;
    render();
  });
  $("[data-retry-cleanup]").addEventListener("click", async () => {
    const button = $("[data-retry-cleanup]");
    button.disabled = true;
    const label = button.textContent;
    button.textContent = "Cleaning up…";
    try { await emitAction("retry-cleanup"); }
    catch (error) { showToast(error instanceof Error ? error.message : "Cleanup still needs attention."); }
    finally { button.disabled = false; button.textContent = label; }
  });
  $("[data-home]").addEventListener("click", () => {
    if (state.snapshot.launch.phase === "running") return;
    closeCatalogue();
    render();
  });

  catalogueToggle.addEventListener("click", () => {
    const expanded = catalogueToggle.getAttribute("aria-expanded") === "true";
    modelDrawer.hidden = expanded;
    catalogueToggle.setAttribute("aria-expanded", expanded ? "false" : "true");
    if (!expanded) modelDrawer.focus({ preventScroll: true });
  });
  modelDrawer.tabIndex = -1;
  modelDrawer.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    event.preventDefault();
    closeCatalogue();
    catalogueToggle.focus({ preventScroll: true });
  });
  $("[data-retry-authority]").addEventListener("click", () => {
    void emitAction("retry-connection").catch((error) => showToast(error instanceof Error ? error.message : "Alzette could not be reached."));
  });
  menuButton.addEventListener("click", () => {
    accountMenu.hidden = !accountMenu.hidden;
    menuButton.setAttribute("aria-expanded", String(!accountMenu.hidden));
  });
  document.addEventListener("click", (event) => {
    if (!accountMenu.hidden && !accountMenu.contains(event.target) && !menuButton.contains(event.target)) {
      accountMenu.hidden = true;
      menuButton.setAttribute("aria-expanded", "false");
    }
  });

  $("[data-diagnostics]").addEventListener("click", () => { accountMenu.hidden = true; diagnosticsDialog.showModal(); });
  $("[data-portal-models]").addEventListener("click", () => { accountMenu.hidden = true; void emitAction("open-portal", "models").catch((error) => showToast(error.message)); });
  $("[data-quit]").addEventListener("click", () => { accountMenu.hidden = true; void emitAction("quit").catch(() => {}); });
  $("[data-sign-out]").addEventListener("click", () => {
    accountMenu.hidden = true;
    if (state.snapshot.launch.phase === "running" || state.snapshot.launch.phase === "preparing") sessionDialog.showModal();
    else void emitAction("sign-out").catch((error) => showToast(error.message));
  });
  $("[data-confirm-sign-out]").addEventListener("click", () => {
    window.setTimeout(() => { void emitAction("sign-out").catch((error) => showToast(error.message)); }, 0);
  });
  $("[data-check-update]").addEventListener("click", () => { void emitAction("check-update").catch((error) => showToast(error.message)); });
  $("[data-install-update]").addEventListener("click", () => { void emitAction("install-update").catch((error) => showToast(error.message)); });

  window.AlzetteConnect = {
    applyState,
    mapRuntimeSnapshot: normalizeSnapshot,
    setNativeMode,
    currentState: () => state.snapshot,
  };
  render();
})();
