let session = JSON.parse(localStorage.getItem("session") || "null");
let streamSession = JSON.parse(localStorage.getItem("streamSession") || "null");
let activeTab = "movies";

const $ = (id) => document.getElementById(id);
const value = (form, key) => new FormData(form).get(key)?.toString().trim() || "";

function log(data) {
  $("log").textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
}

function authHeaders() {
  const headers = {"Content-Type": "application/json"};
  if (session?.token) headers.Authorization = `Bearer ${session.token}`;
  return headers;
}

async function api(path, options = {}) {
  const res = await fetch(path, {...options, headers: {...authHeaders(), ...(options.headers || {})}});
  const text = await res.text();
  const contentType = res.headers.get("content-type") || "";
  let data = {};
  if (text) {
    if (contentType.includes("application/json")) {
      try {
        data = JSON.parse(text);
      } catch {
        data = {message: text};
      }
    } else {
      try {
        data = JSON.parse(text);
      } catch {
        data = {message: text};
      }
    }
  }
  if (!res.ok) throw new Error(data.error || data.message || text || res.statusText);
  return data;
}

function setSession(data) {
  session = {token: data.token, user: data.user};
  localStorage.setItem("session", JSON.stringify(session));
  renderStatus();
}

function renderStatus() {
  $("status").textContent = session?.user ? `${session.user.username} | ${session.user.role || "user"} | ${session.user.id}` : "Not signed in";
  syncAdminVisibility();
}

function isAdmin() {
  return session?.user?.role === "admin";
}

function showTab(tab) {
  if (tab === "admin" && !isAdmin()) {
    log("Admin access required");
    tab = "movies";
  }
  activeTab = tab;
  document.querySelectorAll(".view").forEach((view) => view.classList.add("hidden"));
  $(tab).classList.remove("hidden");
}

function syncAdminVisibility() {
  const adminTab = $("adminTab");
  const adminAllowed = isAdmin();
  adminTab.classList.toggle("hidden", !adminAllowed);
  if (!adminAllowed && activeTab === "admin") {
    showTab("movies");
  }
}

document.querySelectorAll("[data-tab]").forEach((button) => {
  button.addEventListener("click", () => {
    showTab(button.dataset.tab);
  });
});

$("authForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const action = event.submitter.value;
  const body = Object.fromEntries(new FormData(event.currentTarget));
  if (action === "login") delete body.username;
  try {
    const data = await api(`/api/users/users/${action}`, {method: "POST", body: JSON.stringify(body)});
    setSession(data);
    log(data);
    await Promise.allSettled([loadMovies(), loadWatchlist(), loadHistory(), loadRecommendations()]);
  } catch (err) {
    log(err.message);
  }
});

$("logout").addEventListener("click", async () => {
  if (session?.token) {
    try {
      await api("/api/users/users/logout", {method: "POST", body: JSON.stringify({token: session.token})});
    } catch {}
  }
  session = null;
  localStorage.removeItem("session");
  renderStatus();
  showTab("movies");
  log("Logged out");
});

$("profileForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!requireLogin()) return;
  const body = Object.fromEntries(new FormData(event.currentTarget));
  try {
    const data = await api(`/api/users/users/${session.user.id}`, {method: "PATCH", body: JSON.stringify(body)});
    session.user = data.user;
    localStorage.setItem("session", JSON.stringify(session));
    renderStatus();
    log(data);
  } catch (err) {
    log(err.message);
  }
});

$("loadMovies").addEventListener("click", loadMovies);
$("loadTop").addEventListener("click", loadTopMovies);
$("loadGenres").addEventListener("click", loadGenres);
$("loadWatchlist").addEventListener("click", loadWatchlist);
$("loadHistory").addEventListener("click", loadHistory);
$("loadRecommendations").addEventListener("click", loadRecommendations);
$("loadActiveStreams").addEventListener("click", loadActiveStreams);
$("loadPreview").addEventListener("click", loadPreview);

$("searchForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const q = encodeURIComponent(value(event.currentTarget, "q"));
  try {
    const data = await api(`/api/content/movies/search?q=${q}&page=1&limit=20`);
    renderMovies(data.movies || []);
    log(data);
  } catch (err) {
    log(err.message);
  }
});

$("movieForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const action = event.submitter.value;
  const form = event.currentTarget;
  const id = value(form, "movie_id");
  const body = {
    title: value(form, "title"),
    description: value(form, "description"),
    genre_id: value(form, "genre_id"),
    year: Number(value(form, "year") || 0),
    duration_sec: Number(value(form, "duration_sec") || 0),
    video_url: value(form, "video_url"),
    poster_url: value(form, "poster_url")
  };
  try {
    const target = action === "create" ? "/api/content/movies" : `/api/content/movies/${id}`;
    const method = action === "create" ? "POST" : action === "update" ? "PUT" : "DELETE";
    const data = await api(target, {method, body: action === "delete" ? undefined : JSON.stringify(body)});
    log(data);
    await loadMovies();
  } catch (err) {
    log(err.message);
  }
});

$("userAdminForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const id = value(event.currentTarget, "user_id");
  const method = event.submitter.value === "delete" ? "DELETE" : "GET";
  try {
    const data = await api(`/api/users/users/${id}`, {method});
    log(data);
  } catch (err) {
    log(err.message);
  }
});

$("streamForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!requireLogin()) return;
  const action = event.submitter.value;
  const form = event.currentTarget;
  const movieID = value(form, "movie_id");
  const quality = value(form, "quality") || "1080p";
  const position = Number(value(form, "position") || 0);
  try {
    let data;
    if (action === "start") data = await api("/api/streams/streams/start", {method: "POST", body: JSON.stringify({user_id: session.user.id, movie_id: movieID, quality})});
    if (action === "pause") data = await api(`/api/streams/streams/${currentSessionID()}/pause`, {method: "POST"});
    if (action === "resume") data = await api(`/api/streams/streams/${currentSessionID()}/resume`, {method: "POST"});
    if (action === "seek") data = await api(`/api/streams/streams/${currentSessionID()}/seek`, {method: "POST", body: JSON.stringify({position_seconds: position})});
    if (action === "quality") data = await api(`/api/streams/streams/${currentSessionID()}/quality`, {method: "POST", body: JSON.stringify({quality})});
    if (action === "stop") data = await api(`/api/streams/streams/${currentSessionID()}/stop`, {method: "POST", body: JSON.stringify({watched_seconds: position})});
    if (data?.session?.id || data?.session_id || data?.id) {
      streamSession = data.session || data;
      localStorage.setItem("streamSession", JSON.stringify(streamSession));
    }
    log(data);
  } catch (err) {
    log(err.message);
  }
});

$("movieGrid").addEventListener("click", async (event) => {
  const movieID = event.target.dataset.watch || event.target.dataset.history || event.target.dataset.rating || event.target.dataset.similar;
  if (!movieID) return;
  try {
    if (event.target.dataset.watch) {
      if (!requireLogin()) return;
      log(await api(`/api/users/users/${session.user.id}/watchlist`, {method: "POST", body: JSON.stringify({movie_id: movieID})}));
      await loadWatchlist();
    }
    if (event.target.dataset.history) {
      if (!requireLogin()) return;
      log(await api(`/api/users/users/${session.user.id}/history`, {method: "POST", body: JSON.stringify({movie_id: movieID, watched_seconds: 120})}));
      await loadHistory();
    }
    if (event.target.dataset.rating) log(await api(`/api/content/movies/${movieID}/rating`));
    if (event.target.dataset.similar) {
      const data = await api(`/api/content/movies/${movieID}/similar?limit=8`);
      renderMovies(data.movies || []);
      log(data);
    }
  } catch (err) {
    log(err.message);
  }
});

$("genreGrid").addEventListener("click", async (event) => {
  const genreID = event.target.dataset.genre;
  if (!genreID) return;
  try {
    const data = await api(`/api/content/genres/${genreID}/movies?page=1&limit=20`);
    renderMovies(data.movies || []);
    log(data);
  } catch (err) {
    log(err.message);
  }
});

async function loadMovies() {
  try {
    const data = await api("/api/content/movies?page=1&limit=20");
    renderMovies(data.movies || []);
    log(data);
  } catch (err) {
    log(err.message);
  }
}

async function loadTopMovies() {
  try {
    const data = await api("/api/content/movies/top?limit=10");
    renderMovies(data.movies || []);
    log(data);
  } catch (err) {
    log(err.message);
  }
}

async function loadGenres() {
  try {
    const data = await api("/api/content/genres");
    $("genreGrid").innerHTML = (data.genres || []).map((genre) => `<button class="chip" data-genre="${esc(attr(genre.id))}">${esc(genre.name || genre.id)}</button>`).join("");
    log(data);
  } catch (err) {
    log(err.message);
  }
}

async function loadWatchlist() {
  if (!session?.user?.id) return;
  try {
    const data = await api(`/api/users/users/${session.user.id}/watchlist`);
    $("watchlist").innerHTML = renderList(data.items || [], "No watchlist items");
  } catch (err) {
    log(err.message);
  }
}

async function loadHistory() {
  if (!session?.user?.id) return;
  try {
    const data = await api(`/api/users/users/${session.user.id}/history?limit=20`);
    $("history").innerHTML = renderList(data.items || [], "No history items");
  } catch (err) {
    log(err.message);
  }
}

async function loadRecommendations() {
  if (!session?.user?.id) return;
  try {
    const data = await api(`/api/users/users/${session.user.id}/recommendations?limit=10`);
    $("recommendations").innerHTML = (data.movie_ids || data.movieIds || []).map((id) => `<div class="item">${esc(id)}</div>`).join("") || `<div class="item muted">No recommendations yet</div>`;
  } catch (err) {
    log(err.message);
  }
}

async function loadActiveStreams() {
  try {
    const data = await api("/api/streams/streams/active?limit=20");
    $("activeStreams").innerHTML = renderList(data.sessions || data.items || [], "No active streams");
    log(data);
  } catch (err) {
    log(err.message);
  }
}

async function loadPreview() {
  const movieID = value($("streamForm"), "movie_id");
  if (!movieID) {
    log("Enter a movie id first");
    return;
  }
  try {
    const data = await api(`/api/streams/streams/preview?movie_id=${encodeURIComponent(movieID)}`);
    $("streamInfo").innerHTML = `<div class="item">${esc(JSON.stringify(data))}</div>`;
    log(data);
  } catch (err) {
    log(err.message);
  }
}

function renderMovies(movies) {
  $("movieGrid").innerHTML = movies.length ? movies.map((movie) => {
    const id = movie.id || movie.movie_id || movie.movieId || "";
    const duration = movie.duration_sec || movie.durationSec || 0;
    return `
      <article class="card">
        <strong>${esc(movie.title || id)}</strong>
        <div class="muted">${esc(movie.year || "")} ${duration ? `${duration}s` : ""}</div>
        <div class="muted">${esc(movie.description || "")}</div>
        <div class="muted">${esc(id)}</div>
        <div class="actions">
          <button data-watch="${esc(attr(id))}">Watchlist</button>
          <button data-history="${esc(attr(id))}">Watched</button>
        </div>
        <div class="actions">
          <button data-rating="${esc(attr(id))}">Rating</button>
          <button data-similar="${esc(attr(id))}">Similar</button>
        </div>
      </article>
    `;
  }).join("") : `<div class="item muted">No movies found</div>`;
}

function renderList(items, emptyText) {
  return items.length ? items.map((item) => {
    const title = item.title || item.movie_id || item.movieId || item.id || JSON.stringify(item);
    const detail = item.watched_seconds || item.watchedSeconds ? `${item.watched_seconds || item.watchedSeconds}s` : "";
    return `<div class="item"><strong>${esc(title)}</strong><div class="muted">${esc(detail)}</div></div>`;
  }).join("") : `<div class="item muted">${emptyText}</div>`;
}

function currentSessionID() {
  return streamSession?.id || streamSession?.session_id || streamSession?.sessionId || streamSession?.session?.id || "";
}

function requireLogin() {
  if (session?.user?.id) return true;
  log("Login first");
  return false;
}

function esc(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({"&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#039;"}[ch]));
}

function attr(value) {
  return String(value ?? "").replace(/"/g, "&quot;");
}

renderStatus();
showTab(activeTab);
loadMovies();
