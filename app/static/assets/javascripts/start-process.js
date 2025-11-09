(function () {
  "use strict";

  const modal = document.getElementById("start-process-modal");
  if (!modal) {
    return;
  }

  const openButton = document.querySelector('[data-action="open-modal"]');
  const closeSelectors = modal.querySelectorAll('[data-action="close"]');
  const cancelButton = modal.querySelector('[data-action="cancel"]');
  const overlay = modal.querySelector(".modal__overlay");
  const form = modal.querySelector("form");
  const statusEl = document.querySelector('[data-status]');
  const outputEl = document.querySelector('[data-output]');
  const workflowTextarea = modal.querySelector("#workflow-json");
  const workflowFile = modal.querySelector("#workflow-file");
  const tokenInput = modal.querySelector("#token");
  const submitButton = form.querySelector('button[type="submit"]');
  let lastActiveElement = null;

  const status = {
    hide() {
      if (!statusEl) return;
      statusEl.textContent = "";
      statusEl.className = "status";
    },
    show(message, type) {
      if (!statusEl) return;
      statusEl.textContent = message;
      statusEl.className = "status status--visible" + (type ? " status--" + type : "");
    }
  };

  function trapScroll(enable) {
    document.body.style.overflow = enable ? "hidden" : "";
  }

  function openModal() {
    lastActiveElement = document.activeElement;
    modal.classList.add("is-open");
    modal.setAttribute("aria-hidden", "false");
    trapScroll(true);
    window.setTimeout(() => tokenInput && tokenInput.focus(), 0);
  }

  function closeModal() {
    modal.classList.remove("is-open");
    modal.setAttribute("aria-hidden", "true");
    trapScroll(false);
    if (lastActiveElement && typeof lastActiveElement.focus === "function") {
      lastActiveElement.focus();
    }
  }

  openButton && openButton.addEventListener("click", openModal);

  closeSelectors.forEach((el) => {
    el.addEventListener("click", closeModal);
  });

  cancelButton && cancelButton.addEventListener("click", function (event) {
    event.preventDefault();
    closeModal();
  });

  overlay && overlay.addEventListener("click", closeModal);

  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape" && modal.classList.contains("is-open")) {
      closeModal();
    }
  });

  workflowFile && workflowFile.addEventListener("change", function (event) {
    const file = event.target.files && event.target.files[0];
    if (!file) {
      return;
    }
    const reader = new FileReader();
    reader.onload = function () {
      if (typeof reader.result === "string") {
        workflowTextarea.value = reader.result;
      }
    };
    reader.readAsText(file);
  });

  async function submitWorkflow(event) {
    event.preventDefault();
    status.hide();
    outputEl && (outputEl.value = "");

    let configText = workflowTextarea.value.trim();
    if (!configText) {
      status.show("Workflow JSON is required.", "error");
      return;
    }

    let parsedConfig;
    try {
      parsedConfig = JSON.parse(configText);
    } catch (err) {
      status.show("Workflow JSON could not be parsed: " + err.message, "error");
      return;
    }

    const hostInput = form.querySelector('#host');
    const portInput = form.querySelector('#port');
    const outputInput = form.querySelector('#output-path');

    const hostValue = hostInput && hostInput.value.trim();
    const portValue = portInput && portInput.value.trim();
    const outputValue = outputInput && outputInput.value.trim();
    const tokenValue = tokenInput && tokenInput.value.trim();

    if (hostValue) {
      parsedConfig.Host = hostValue;
    }
    if (portValue) {
      const numericPort = Number(portValue);
      if (!Number.isFinite(numericPort) || numericPort <= 0 || numericPort > 65535) {
        status.show("Port must be a number between 1 and 65535.", "error");
        return;
      }
      parsedConfig.Port = numericPort;
    }

    if (!parsedConfig.Host || !parsedConfig.Port) {
      status.show("Host and port are required either in the form or within the workflow JSON.", "error");
      return;
    }

    if (outputValue) {
      parsedConfig.OutputFilePath = outputValue;
    }

    if (tokenValue) {
      parsedConfig.Token = tokenValue;
    } else {
      delete parsedConfig.Token;
    }

    if (!Array.isArray(parsedConfig.Steps) || parsedConfig.Steps.length === 0) {
      status.show("Workflow JSON must include a non-empty Steps array.", "error");
      return;
    }

    submitButton.disabled = true;
    status.show("Submitting workflow…", "pending");

    try {
      const response = await fetch("/api/execute", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(parsedConfig)
      });

      const payloadText = await response.text();
      let payload = null;
      try {
        payload = payloadText ? JSON.parse(payloadText) : null;
      } catch (err) {
        payload = null;
      }

      if (!response.ok) {
        const message = payload && payload.message ? payload.message : response.statusText;
        status.show("Workflow failed: " + message, "error");
        return;
      }

      status.show("Workflow executed successfully.", "success");
      if (payload && payload.output && outputEl) {
        outputEl.value = payload.output;
      }
    } catch (err) {
      status.show("Unable to reach the API server: " + err.message, "error");
    } finally {
      submitButton.disabled = false;
    }
  }

  form.addEventListener("submit", submitWorkflow);
})();
