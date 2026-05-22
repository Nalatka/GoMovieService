// ── State ──────────────────────────────────────────────────────────────────
let session        = JSON.parse(localStorage.getItem("session")       || "null");
let streamSession  = JSON.parse(localStorage.getItem("streamSession") || "null");
let activeTab      = "movies";
let currentMovie   = null;
let streamTimer    = null;
let watchedSeconds = 0;
let resumePosition = 0;
let ytPlayer       = null;   // YouTube IFrame API player instance
let ytReady        = false;  // API loaded flag

// Load YouTube IFrame API once
(function () {
  const tag = document.createElement("script");
  tag.src   = "https://www.youtube.com/iframe_api";
  document.head.appendChild(tag);
})();

window.onYouTubeIframeAPIReady = () => { ytReady = true; };

const $ = (id) => document.getElementById(id);

// ── Logging ─────────────────────────────────────────────────────────────────
function log(data) {
  $("log").textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
}

// ── API helper ──────────────────────────────────────────────────────────────
function authHeaders() {
  const h = {"Content-Type": "application/json"};
  if (session?.token) h.Authorization = `Bearer ${session.token}`;
  return h;
}

async function api(path, options = {}) {
  const res  = await fetch(path, {...options, headers: {...authHeaders(), ...(options.headers || {})}});
  const text = await res.text();
  let data   = {};
  if (text) {
    try { data = JSON.parse(text); } catch { data = {message: text}; }
  }
  if (!res.ok) throw new Error(data.error || data.message || text || res.statusText);
  return data;
}

// ── Auth ────────────────────────────────────────────────────────────────────
function setSession(data) {
  session = {token: data.token, user: data.user};
  localStorage.setItem("session", JSON.stringify(session));
  renderStatus();
}

function renderStatus() {
  $("status").textContent = session?.user
    ? `${session.user.username} | ${session.user.role || "user"} | ${session.user.id}`
    : "Not signed in";
  $("adminTab").classList.toggle("hidden", session?.user?.role !== "admin");
}

function isAdmin() { return session?.user?.role === "admin"; }

function requireLogin() {
  if (session?.user?.id) return true;
  log("Please login first");
  return false;
}

// ── Tabs ────────────────────────────────────────────────────────────────────
function showTab(tab) {
  if (tab === "admin" && !isAdmin()) { tab = "movies"; }
  activeTab = tab;
  document.querySelectorAll(".view").forEach(v => v.classList.add("hidden"));
  $(tab)?.classList.remove("hidden");
}

document.querySelectorAll("[data-tab]").forEach(btn =>
  btn.addEventListener("click", () => showTab(btn.dataset.tab))
);

// ── Auth form ───────────────────────────────────────────────────────────────
$("authForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const action = e.submitter.value;
  const body   = Object.fromEntries(new FormData(e.currentTarget));
  if (action === "login") delete body.username;
  try {
    const data = await api(`/api/users/users/${action}`, {method: "POST", body: JSON.stringify(body)});
    setSession(data);
    log(data);
    loadMovies();
    loadWatchlist();
    loadHistory();
  } catch (err) { log(err.message); }
});

$("logout").addEventListener("click", async () => {
  if (session?.token) {
    try { await api("/api/users/users/logout", {method: "POST", body: JSON.stringify({token: session.token})}); } catch {}
  }
  session = null;
  localStorage.removeItem("session");
  renderStatus();
  showTab("movies");
  log("Logged out");
});

// ── Profile ─────────────────────────────────────────────────────────────────
$("profileForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!requireLogin()) return;
  const body = Object.fromEntries(new FormData(e.currentTarget));
  try {
    const data = await api(`/api/users/users/${session.user.id}`, {method: "PATCH", body: JSON.stringify(body)});
    session.user = data.user;
    localStorage.setItem("session", JSON.stringify(session));
    renderStatus();
    log(data);
  } catch (err) { log(err.message); }
});

// ── Movies ──────────────────────────────────────────────────────────────────
$("loadMovies").addEventListener("click", loadMovies);
$("loadTop").addEventListener("click",    loadTopMovies);
$("loadGenres").addEventListener("click", loadGenres);

$("searchForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const q = encodeURIComponent(new FormData(e.currentTarget).get("q")?.toString().trim() || "");
  try {
    const data = await api(`/api/content/movies/search?q=${q}&page=1&limit=40`);
    renderMovies(data.movies || []);
    log(data);
  } catch (err) { log(err.message); }
});

async function loadMovies() {
  try {
    const data = await api("/api/content/movies?page=1&limit=40");
    renderMovies(data.movies || []);
  } catch (err) { log(err.message); }
}

async function loadTopMovies() {
  try {
    const data = await api("/api/content/movies/top?limit=20");
    renderMovies(data.movies || []);
  } catch (err) { log(err.message); }
}

async function loadGenres() {
  try {
    const data = await api("/api/content/genres");
    $("genreGrid").innerHTML = (data.genres || [])
      .map(g => `<button class="chip" data-genre="${attr(g.id)}">${esc(g.name || g.id)}</button>`)
      .join("");
  } catch (err) { log(err.message); }
}

$("genreGrid").addEventListener("click", async (e) => {
  const gid = e.target.dataset.genre;
  if (!gid) return;
  try {
    const data = await api(`/api/content/genres/${gid}/movies?page=1&limit=40`);
    renderMovies(data.movies || []);
  } catch (err) { log(err.message); }
});

// ── Movie card rendering ─────────────────────────────────────────────────────
function renderMovies(movies) {
  if (!movies.length) {
    $("movieGrid").innerHTML = `<div style="color:var(--muted);padding:20px">No movies found</div>`;
    return;
  }
  $("movieGrid").innerHTML = movies.map(m => {
    const id      = m.id || m.movie_id || "";
    const rating  = m.avg_rating ? `⭐ ${Number(m.avg_rating).toFixed(1)}` : "";
    const poster  = m.poster_url || m.posterUrl || "";
    const imgTag  = poster
      ? `<img class="poster" src="${attr(poster)}" alt="${attr(m.title)}" loading="lazy" onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">`
      : "";
    const placeholder = `<div class="poster-placeholder" style="${poster ? "display:none" : ""}">🎬</div>`;
    return `
      <div class="movie-card" data-id="${attr(id)}">
        ${imgTag}${placeholder}
        <div class="card-body">
          <div class="card-title">${esc(m.title || id)}</div>
          <div class="card-meta">${esc(m.year || "")}  ${rating}</div>
        </div>
      </div>`;
  }).join("");
}

// Click on movie card → open modal
$("movieGrid").addEventListener("click", async (e) => {
  const card = e.target.closest(".movie-card");
  if (!card) return;
  const movieId = card.dataset.id;
  try {
    const data = await api(`/api/content/movies/${movieId}`);
    openModal(data.movie || data);
    log(data);
  } catch (err) { log(err.message); }
});

// ── Library ──────────────────────────────────────────────────────────────────
$("loadWatchlist").addEventListener("click",      loadWatchlist);
$("loadHistory").addEventListener("click",        loadHistory);
$("loadRecommendations").addEventListener("click", loadRecommendations);

// Watch button inside library lists
document.getElementById("library").addEventListener("click", async (e) => {
  const mid = e.target.dataset.openMovie;
  if (!mid) return;
  try {
    const data = await api(`/api/content/movies/${mid}`);
    openModal(data.movie || data);
  } catch (err) { log(err.message); }
});

async function loadWatchlist() {
  if (!session?.user?.id) return;
  try {
    const data  = await api(`/api/users/users/${session.user.id}/watchlist`);
    const items = data.items || [];
    $("watchlist").innerHTML = items.length
      ? "<div style='color:var(--muted);font-size:12px;margin-bottom:8px'>Loading details…</div>"
      : `<div class="list-item" style="color:var(--muted)">No watchlist items</div>`;

    if (!items.length) return;

    const enriched = await Promise.allSettled(
      items.map(item => {
        const mid = item.movie_id || item.movieId || "";
        return api(`/api/content/movies/${mid}`).then(d => ({...item, _movie: d.movie || d}));
      })
    );
    const rows = enriched.map(r => r.status === "fulfilled" ? r.value : null).filter(Boolean);
    $("watchlist").innerHTML = rows.map(item => renderLibraryItem(item._movie, item.movie_id || item.movieId || "", "")).join("");
  } catch (err) { log(err.message); }
}

async function loadHistory() {
  if (!session?.user?.id) return;
  try {
    const data  = await api(`/api/users/users/${session.user.id}/history?limit=20`);
    const items = data.items || [];
    $("history").innerHTML = items.length
      ? "<div style='color:var(--muted);font-size:12px;margin-bottom:8px'>Loading details…</div>"
      : `<div class="list-item" style="color:var(--muted)">No history yet</div>`;

    if (!items.length) return;

    const enriched = await Promise.allSettled(
      items.map(item => {
        const mid = item.movie_id || item.movieId || "";
        return api(`/api/content/movies/${mid}`).then(d => ({...item, _movie: d.movie || d}));
      })
    );
    const rows = enriched.map(r => r.status === "fulfilled" ? r.value : null).filter(Boolean);
    $("history").innerHTML = rows.map(item => {
      const secs = item.watched_seconds || item.watchedSeconds || 0;
      return renderLibraryItem(item._movie, item.movie_id || item.movieId || "", secs ? `Watched: ${formatTime(secs)}` : "");
    }).join("");
  } catch (err) { log(err.message); }
}

function renderLibraryItem(movie, fallbackId, meta) {
  const id     = (movie && (movie.id || movie.movie_id)) || fallbackId;
  const title  = (movie && movie.title) || fallbackId;
  const year   = (movie && movie.year)  || "";
  const poster = (movie && (movie.poster_url || movie.posterUrl)) || "";
  return `<div class="list-item" style="display:flex;gap:12px;align-items:center">
    ${poster ? `<img src="${attr(poster)}" style="width:40px;height:58px;object-fit:cover;border-radius:4px;flex-shrink:0">` : `<div style="width:40px;height:58px;background:#1e1e2e;border-radius:4px;flex-shrink:0;display:flex;align-items:center;justify-content:center;font-size:18px">🎬</div>`}
    <div style="flex:1;min-width:0">
      <div class="item-title" style="white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${esc(title)}</div>
      <div class="item-meta">${esc(year)}${meta ? " · " + esc(meta) : ""}</div>
    </div>
    <button data-open-movie="${attr(id)}" style="font-size:12px;padding:6px 12px;flex-shrink:0">▶ Watch</button>
  </div>`;
}

async function loadRecommendations() {
  if (!session?.user?.id) return;
  try {
    const data  = await api(`/api/users/users/${session.user.id}/recommendations?limit=10`);
    const ids   = data.movie_ids || data.movieIds || [];
    $("recommendations").innerHTML = ids.length
      ? ids.map(id => `<div class="list-item"><div class="item-title">${esc(id)}</div></div>`).join("")
      : `<div class="list-item" style="color:var(--muted)">No recommendations yet</div>`;
  } catch (err) { log(err.message); }
}

function renderList(items, emptyText) {
  if (!items.length) return `<div class="list-item" style="color:var(--muted)">${esc(emptyText)}</div>`;
  return items.map(item => {
    const title  = item.title || item.movie_id || item.movieId || item.id || JSON.stringify(item);
    const detail = (item.watched_seconds || item.watchedSeconds) ? `Watched: ${item.watched_seconds || item.watchedSeconds}s` : "";
    return `<div class="list-item">
      <div class="item-title">${esc(title)}</div>
      ${detail ? `<div class="item-meta">${esc(detail)}</div>` : ""}
    </div>`;
  }).join("");
}

// ── Admin ────────────────────────────────────────────────────────────────────
$("movieForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const action = e.submitter.value;
  const form   = e.currentTarget;
  const id     = new FormData(form).get("movie_id")?.toString().trim() || "";
  const body   = {
    title:        new FormData(form).get("title")?.toString().trim(),
    description:  new FormData(form).get("description")?.toString().trim(),
    genre_id:     new FormData(form).get("genre_id")?.toString().trim(),
    year:         Number(new FormData(form).get("year") || 0),
    duration_sec: Number(new FormData(form).get("duration_sec") || 0),
    video_url:    new FormData(form).get("video_url")?.toString().trim(),
    poster_url:   new FormData(form).get("poster_url")?.toString().trim(),
  };
  try {
    const target = action === "create" ? "/api/content/movies" : `/api/content/movies/${id}`;
    const method = action === "create" ? "POST" : action === "update" ? "PUT" : "DELETE";
    const data   = await api(target, {method, body: action === "delete" ? undefined : JSON.stringify(body)});
    log(data);
    loadMovies();
  } catch (err) { log(err.message); }
});

$("userAdminForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const id     = new FormData(e.currentTarget).get("user_id")?.toString().trim() || "";
  const method = e.submitter.value === "delete" ? "DELETE" : "GET";
  try {
    log(await api(`/api/users/users/${id}`, {method}));
  } catch (err) { log(err.message); }
});

// ── Player Modal ─────────────────────────────────────────────────────────────
function formatTime(sec) {
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

async function checkExistingSession(movieId) {
  if (!movieId) return null;

  // First: check locally stored session for this exact movie
  if (streamSession) {
    const storedMid = streamSession.movie_id || streamSession.movieId || "";
    if (storedMid === movieId) {
      const sid = streamSession.id || streamSession.session_id || "";
      if (sid) {
        try {
          const data = await api(`/api/streams/streams/${sid}`);
          const sess = data.session || data;
          if ((sess.status || 0) !== 3) return sess; // not completed
        } catch {}
      }
    }
  }

  // Fallback: scan active sessions from server
  if (!session?.user?.id) return null;
  try {
    const data = await api("/api/streams/streams/active?limit=100");
    const sessions = data.sessions || data.items || [];
    return sessions.find(s => {
      const mid = s.movie_id || s.movieId || "";
      const uid = s.user_id  || s.userId  || "";
      return mid === movieId && uid === session.user.id && (s.status || 0) !== 3;
    }) || null;
  } catch { return null; }
}

function extractYouTubeID(url) {
  if (!url) return null;
  const m = url.match(/(?:youtu\.be\/|[?&]v=)([A-Za-z0-9_-]{11})/);
  return m ? m[1] : null;
}

async function openModal(movie) {
  currentMovie   = movie;
  watchedSeconds = 0;
  resumePosition = 0;

  const id      = movie.id || movie.movie_id || "";
  const ytID    = extractYouTubeID(movie.video_url || movie.videoUrl || "");
  const rating  = movie.avg_rating ? `⭐ ${Number(movie.avg_rating).toFixed(1)}` : "Not rated yet";
  const views   = movie.views ? `${movie.views} views` : "";

  $("modalTitle").textContent = movie.title || "Untitled";
  $("modalMeta").textContent  = [movie.year, movie.genre, rating, views].filter(Boolean).join(" · ");
  $("modalDesc").textContent  = movie.description || "";

  $("playerFrame").innerHTML = ytID
    ? `<div class="no-video" style="height:100%">Click <strong>Watch</strong> to start streaming</div>`
    : `<div class="no-video">No video available for this movie</div>`;

  setStreamUI("idle");
  $("playerModal").classList.remove("hidden");
  document.body.style.overflow = "hidden";

  // Check for existing unfinished session
  const existing = await checkExistingSession(id);
  if (existing) {
    const pos = existing.position_seconds || existing.positionSeconds || 0;
    streamSession = existing;
    localStorage.setItem("streamSession", JSON.stringify(streamSession));
    watchedSeconds = pos;
    resumePosition = pos;
    if (pos > 0) {
      $("modalWatch").textContent = `▶ Resume from ${formatTime(pos)}`;
    }
    // If session was paused, reflect that in UI
    if (existing.status === 2) setStreamUI("paused");
  }

  // Watchlist button
  $("modalWatchlist").onclick = async () => {
    if (!requireLogin()) return;
    try {
      await api(`/api/users/users/${session.user.id}/watchlist`, {
        method: "POST", body: JSON.stringify({movie_id: id})
      });
      log("Added to watchlist");
    } catch (err) { log(err.message); }
  };

  // Rating
  $("ratingSlider").oninput = () => { $("ratingValue").textContent = $("ratingSlider").value; };
  $("submitRating").onclick = async () => {
    try {
      const score = Number($("ratingSlider").value);
      const data  = await api(`/api/content/movies/${id}/rate`, {
        method: "POST", body: JSON.stringify({movie_id: id, score})
      });
      log(data);
    } catch (err) { log(err.message); }
  };

  // Watch / Resume button
  $("modalWatch").onclick = async () => {
    if (!requireLogin()) return;
    if (!ytID) { log("No YouTube video URL for this movie"); return; }
    if (!ytReady) { log("Player not ready yet, try again"); return; }

    try {
      const sid = currentSessionID();

      if (sid && resumePosition > 0) {
        await api(`/api/streams/streams/${sid}/resume`, {method: "POST"});
      } else {
        const data = await api("/api/streams/streams/start", {
          method: "POST",
          body: JSON.stringify({user_id: session.user.id, movie_id: id, quality: "1080p"})
        });
        streamSession = data.session || data;
        localStorage.setItem("streamSession", JSON.stringify(streamSession));
        log(data);
      }

      startYTPlayer(ytID, watchedSeconds);
      $("modalWatch").textContent = "▶ Watch";
      resumePosition = 0;
      setStreamUI("playing");
    } catch (err) { log(err.message); }
  };

  // Pause — pauses actual video + saves position to stream service
  $("modalPause").onclick = async () => {
    if (ytPlayer) ytPlayer.pauseVideo();
    await syncPause();
  };

  // Resume — resumes actual video
  $("modalResume").onclick = async () => {
    const sid = currentSessionID();
    if (!sid) return;
    try {
      if (ytPlayer) ytPlayer.playVideo();
      await api(`/api/streams/streams/${sid}/resume`, {method: "POST"});
      setStreamUI("playing");
    } catch (err) { log(err.message); }
  };

  // Stop
  $("modalStop").onclick = () => stopStream();
}

function startYTPlayer(videoId, startSec) {
  // Destroy previous player if any
  if (ytPlayer) { try { ytPlayer.destroy(); } catch {} ytPlayer = null; }
  clearInterval(streamTimer);

  $("playerFrame").innerHTML = `<div id="yt-container" style="width:100%;height:100%"></div>`;

  ytPlayer = new YT.Player("yt-container", {
    videoId,
    playerVars: { autoplay: 1, start: Math.floor(startSec || 0), rel: 0 },
    width: "100%",
    height: "100%",
    events: {
      onReady(e) { e.target.playVideo(); startSyncTimer(); },
      onStateChange(e) {
        // User paused/played inside the YouTube player
        if (e.data === YT.PlayerState.PAUSED)  syncPause();
        if (e.data === YT.PlayerState.PLAYING)  setStreamUI("playing");
        if (e.data === YT.PlayerState.ENDED)    stopStream();
      }
    }
  });
}

function startSyncTimer() {
  clearInterval(streamTimer);
  streamTimer = setInterval(() => {
    watchedSeconds = ytPlayer ? Math.floor(ytPlayer.getCurrentTime() || 0) : watchedSeconds + 1;
    $("streamStatus").textContent = `🔴 Streaming — ${formatTime(watchedSeconds)}`;
    if (watchedSeconds > 0 && watchedSeconds % 30 === 0) {
      const sid = currentSessionID();
      if (sid) {
        api(`/api/streams/streams/${sid}/seek`, {
          method: "POST",
          body: JSON.stringify({position_seconds: watchedSeconds})
        }).catch(() => {});
      }
    }
  }, 1000);
}

async function syncPause() {
  clearInterval(streamTimer);
  const sid = currentSessionID();
  if (!sid) return;
  const pos = ytPlayer ? Math.floor(ytPlayer.getCurrentTime() || 0) : watchedSeconds;
  watchedSeconds = pos;
  try {
    await api(`/api/streams/streams/${sid}/pause`, {
      method: "POST",
      body: JSON.stringify({position_seconds: pos})
    });
    setStreamUI("paused");
    $("streamStatus").textContent = `⏸ Paused at ${formatTime(pos)}`;
  } catch {}
}

function setStreamUI(state) {
  const status  = $("streamStatus");
  const watch   = $("modalWatch");
  const pause   = $("modalPause");
  const resume  = $("modalResume");
  const stop    = $("modalStop");

  watch.classList.toggle("hidden",  state !== "idle");
  pause.classList.toggle("hidden",  state !== "playing");
  resume.classList.toggle("hidden", state !== "paused");
  stop.classList.toggle("hidden",   state === "idle");

  if (state === "idle")    { status.textContent = "No active stream";  status.className = "stream-status"; }
  if (state === "playing") { status.textContent = "🔴 Streaming...";   status.className = "stream-status active"; }
  if (state === "paused")  { status.textContent = "⏸ Paused";          status.className = "stream-status"; }
}

async function stopStream() {
  clearInterval(streamTimer);

  // Get final position from player before destroying it
  if (ytPlayer) {
    try { watchedSeconds = Math.floor(ytPlayer.getCurrentTime() || watchedSeconds); } catch {}
    try { ytPlayer.destroy(); } catch {}
    ytPlayer = null;
  }

  const sid = currentSessionID();
  if (sid) {
    try {
      await api(`/api/streams/streams/${sid}/stop`, {method: "POST"});
      if (session?.user?.id && currentMovie) {
        await api(`/api/users/users/${session.user.id}/history`, {
          method: "POST",
          body: JSON.stringify({
            movie_id: currentMovie.id || currentMovie.movie_id,
            watched_seconds: watchedSeconds
          })
        });
      }
    } catch {}
  }

  streamSession = null;
  localStorage.removeItem("streamSession");
  setStreamUI("idle");
  $("playerFrame").innerHTML = `<div class="no-video" style="height:100%">Stream stopped</div>`;
}

function closeModal() {
  stopStream().finally(() => {
    currentMovie = null;
    $("playerModal").classList.add("hidden");
    document.body.style.overflow = "";
  });
}

$("closeModal").addEventListener("click", closeModal);
$("playerModal").addEventListener("click", (e) => {
  if (e.target === $("playerModal")) closeModal();
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") closeModal();
});

// ── Helpers ──────────────────────────────────────────────────────────────────
function currentSessionID() {
  return streamSession?.id || streamSession?.session_id || streamSession?.session?.id || "";
}

function esc(v) {
  return String(v ?? "").replace(/[&<>"']/g, c =>
    ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#039;"}[c])
  );
}

function attr(v) {
  return String(v ?? "").replace(/"/g, "&quot;");
}

// ── Init ─────────────────────────────────────────────────────────────────────
renderStatus();
showTab("movies");
loadMovies();
