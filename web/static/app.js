const dropZone = document.querySelector("#dropZone");
const fileInput = document.querySelector("#fileInput");
const errorEl = document.querySelector("#error");

const workspace = document.querySelector("#workspace");
const photoCountEl = document.querySelector("#photoCount");
const startOverBtn = document.querySelector("#startOver");
const cleanBtn = document.querySelector("#cleanBtn");
const cleanStatus = document.querySelector("#cleanStatus");
const photoGrid = document.querySelector("#photoGrid");

let batchId = null;
let photos = []; // [{id, filename, info, error, file, cleaned, cleanedFilename, cleanedPath}]

function showError(message) {
  errorEl.textContent = message;
  errorEl.classList.remove("hidden");
}
function clearError() {
  errorEl.classList.add("hidden");
  errorEl.textContent = "";
}

function resetToDropZone() {
  batchId = null;
  photos = [];
  photoGrid.innerHTML = "";
  fileInput.value = "";
  workspace.classList.add("hidden");
  dropZone.classList.remove("hidden");
  clearError();
}

// ---- drop zone ------------------------------------------------------------

dropZone.addEventListener("click", () => fileInput.click());
fileInput.addEventListener("change", () => handleFiles(fileInput.files));

["dragenter", "dragover"].forEach((evt) =>
  dropZone.addEventListener(evt, (e) => {
    e.preventDefault();
    dropZone.classList.add("dragover");
  })
);
["dragleave", "drop"].forEach((evt) =>
  dropZone.addEventListener(evt, (e) => {
    e.preventDefault();
    dropZone.classList.remove("dragover");
  })
);
dropZone.addEventListener("drop", (e) => {
  if (e.dataTransfer?.files?.length) handleFiles(e.dataTransfer.files);
});

async function handleFiles(fileList) {
  const files = Array.from(fileList).filter(
    (f) => f.type === "image/jpeg" || f.type === "image/png" || /\.(jpe?g|png)$/i.test(f.name)
  );
  if (files.length === 0) {
    showError("Only JPEG and PNG photos are supported.");
    return;
  }
  clearError();

  const form = new FormData();
  for (const file of files) form.append("file", file, file.name);

  let data;
  try {
    const res = await fetch("/api/upload", { method: "POST", body: form });
    data = await res.json();
    if (!res.ok) throw new Error(data.error || "upload failed");
  } catch (err) {
    showError(String(err.message || err));
    return;
  }

  batchId = data.batchId;
  photos = data.photos.map((p, i) => ({ ...p, file: files[i], cleaned: false }));

  dropZone.classList.add("hidden");
  workspace.classList.remove("hidden");
  renderGrid();
}

// ---- rendering ------------------------------------------------------------

function renderGrid() {
  photoCountEl.textContent = `${photos.length} photo${photos.length === 1 ? "" : "s"}`;
  photoGrid.innerHTML = "";
  photos.forEach((p) => photoGrid.appendChild(renderCard(p)));
  updateCleanStatus();
}

function renderCard(p) {
  const card = document.createElement("div");
  card.className = "photo-card";
  card.dataset.id = p.id;

  const img = document.createElement("img");
  img.className = "photo-thumb";
  img.src = URL.createObjectURL(p.file);
  img.alt = "";
  card.appendChild(img);

  const body = document.createElement("div");
  body.className = "photo-body";

  const top = document.createElement("div");
  top.className = "photo-top";

  const checkbox = document.createElement("input");
  checkbox.type = "checkbox";
  checkbox.checked = true;
  checkbox.addEventListener("change", updateCleanStatus);
  top.appendChild(checkbox);

  const name = document.createElement("span");
  name.className = "photo-name";
  name.textContent = p.filename;
  name.title = p.filename;
  top.appendChild(name);
  body.appendChild(top);

  if (p.error) {
    const err = document.createElement("div");
    err.className = "photo-error";
    err.textContent = p.error;
    body.appendChild(err);
  } else {
    body.appendChild(renderFields(p.info));
  }

  const actions = document.createElement("div");
  actions.className = "photo-actions";
  body.appendChild(actions);

  card.appendChild(body);
  card._checkbox = checkbox;
  card._actions = actions;
  return card;
}

function renderFields(info) {
  const wrap = document.createElement("div");
  if (!info || !info.hasMetadata) {
    wrap.className = "no-metadata";
    wrap.textContent = "No metadata found in this file.";
    return wrap;
  }

  wrap.className = "photo-fields";
  const rows = [];

  if (info.hasGps) {
    const row = document.createElement("div");
    row.className = "field-row";
    const label = document.createElement("span");
    label.className = "field-label";
    label.textContent = "Location";
    const value = document.createElement("span");
    value.className = "field-value";
    const link = document.createElement("a");
    link.href = `https://www.google.com/maps?q=${info.latitude},${info.longitude}`;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = `${info.latitude.toFixed(5)}, ${info.longitude.toFixed(5)} — view on map`;
    value.appendChild(link);
    row.append(label, value);
    rows.push(row);
  }
  if (info.make || info.model) {
    rows.push(fieldRow("Camera", [info.make, info.model].filter(Boolean).join(" ")));
  }
  if (info.dateTime) {
    rows.push(fieldRow("Date", info.dateTime));
  }
  if (info.software) {
    rows.push(fieldRow("Software", info.software));
  }

  if (rows.length === 0) {
    wrap.className = "no-metadata";
    wrap.textContent = "No metadata found in this file.";
    return wrap;
  }

  rows.forEach((r) => wrap.appendChild(r));
  return wrap;
}

function fieldRow(label, value) {
  const row = document.createElement("div");
  row.className = "field-row";
  const l = document.createElement("span");
  l.className = "field-label";
  l.textContent = label;
  const v = document.createElement("span");
  v.className = "field-value";
  v.textContent = value;
  v.title = value;
  row.append(l, v);
  return row;
}

function updateCleanStatus() {
  const selected = photos.filter((p) => !p.error && !p.cleaned && cardFor(p.id)?._checkbox?.checked);
  cleanBtn.disabled = selected.length === 0;
  cleanStatus.textContent = selected.length === 0 ? "" : `${selected.length} selected`;
}

function cardFor(id) {
  return photoGrid.querySelector(`.photo-card[data-id="${id}"]`);
}

// ---- clean ------------------------------------------------------------

cleanBtn.addEventListener("click", async () => {
  const ids = photos
    .filter((p) => !p.error && !p.cleaned && cardFor(p.id)?._checkbox?.checked)
    .map((p) => p.id);
  if (ids.length === 0) return;

  cleanBtn.disabled = true;
  cleanStatus.textContent = "Removing private information…";

  try {
    const res = await fetch("/api/clean", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ batchId, ids }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "cleaning failed");

    for (const r of data.results) {
      const photo = photos.find((p) => p.id === r.id);
      if (!photo) continue;
      if (r.error) {
        photo.error = r.error;
      } else {
        photo.cleaned = true;
        photo.cleanedFilename = r.filename;
        photo.cleanedPath = r.path;
      }
      updateCard(photo);
    }
    cleanStatus.textContent = `Saved ${data.results.filter((r) => !r.error).length} clean cop${data.results.filter((r) => !r.error).length === 1 ? "y" : "ies"} to Downloads.`;
  } catch (err) {
    showError(String(err.message || err));
    cleanStatus.textContent = "";
  } finally {
    updateCleanStatus();
  }
});

function updateCard(photo) {
  const card = cardFor(photo.id);
  if (!card) return;

  if (photo.cleaned) {
    card.classList.add("cleaned");
    card._checkbox.checked = false;
    card._checkbox.disabled = true;

    const badge = document.createElement("div");
    badge.className = "cleaned-badge";
    badge.textContent = `✓ Cleaned — saved as ${photo.cleanedFilename}`;
    card.querySelector(".photo-body").insertBefore(badge, card._actions);

    const openBtn = document.createElement("button");
    openBtn.type = "button";
    openBtn.className = "link-btn";
    openBtn.textContent = "Open file";
    openBtn.addEventListener("click", () => {
      fetch("/api/open", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path: photo.cleanedPath }) });
    });
    const revealBtn = document.createElement("button");
    revealBtn.type = "button";
    revealBtn.className = "link-btn";
    revealBtn.textContent = "Show in folder";
    revealBtn.addEventListener("click", () => {
      fetch("/api/reveal", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path: photo.cleanedPath }) });
    });
    card._actions.append(openBtn, revealBtn);
  } else if (photo.error) {
    const body = card.querySelector(".photo-body");
    const existing = body.querySelector(".photo-error");
    if (existing) existing.textContent = photo.error;
  }
}

// ---- start over ------------------------------------------------------------

startOverBtn.addEventListener("click", resetToDropZone);
