/* ============================================================
   AutoPartStore — script.js
   ============================================================

   CORS — ВАЖНО ДЛЯ Go-СЕРВЕРА:
   ─────────────────────────────────────────────────────────────
   GET  /api/parts     — возвращает список запчастей
   POST /api/parts/add — добавляет новую запчасть

   Go-сервер обязан возвращать заголовки:
       Access-Control-Allow-Origin:  *
       Access-Control-Allow-Methods: GET, POST, OPTIONS
       Access-Control-Allow-Headers: Content-Type
   ─────────────────────────────────────────────────────────────
   Структура JSON:
     GET  ← [{"id":1,"name":"...","article":"...","price":1000}]
     POST → {"name":"...","article":"...","price":1000}
   ============================================================ */

'use strict';

/* ─── 1. КОНФИГУРАЦИЯ ───────────────────────────────────────── */

const API_BASE  = 'http://localhost:8080';
const API_PARTS = `${API_BASE}/api/parts`;
const API_ADD   = `${API_BASE}/api/parts/add`;

const FALLBACK_ICONS = ['🔧', '⚙️', '⛽', '🌡️', '⚡', '💨', '🔩', '🚗', '🛞', '🪛'];

/* ─── 2. СОСТОЯНИЕ ──────────────────────────────────────────── */

/** Данные с сервера. Поиск фильтрует по этому массиву. */
let serverParts = [];

/* ─── 3. DOM REFS ───────────────────────────────────────────── */

const grid         = document.getElementById('productGrid');
const searchInput  = document.getElementById('searchInput');
const searchBtn    = document.getElementById('searchBtn');
const noResults    = document.getElementById('noResults');
const resetBtn     = document.getElementById('resetBtn');
const resultsCount = document.getElementById('resultsCount');
const burger       = document.getElementById('burger');
const mobileNav    = document.getElementById('mobileNav');
const quickTags    = document.querySelectorAll('.quick-tag');

// Admin form
const newName      = document.getElementById('newName');
const newArticle   = document.getElementById('newArticle');
const newPrice     = document.getElementById('newPrice');
const addBtn       = document.getElementById('addBtn');
const formStatus   = document.getElementById('formStatus');

/* ─── 4. ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ───────────────────────────── */

function formatPrice(n) {
  return Number(n).toLocaleString('ru-RU') + ' ₽';
}

function updateCount(n) {
  resultsCount.innerHTML = `Показано товаров: <strong>${n}</strong>`;
}

/** Скелетон — заглушка на время загрузки */
function showSkeleton() {
  grid.innerHTML = '';
  noResults.classList.add('hidden');
  for (let i = 0; i < 8; i++) {
    const ghost = document.createElement('article');
    ghost.className = 'product-card skeleton-card';
    ghost.innerHTML = `
      <div class="img-placeholder skeleton-pulse"></div>
      <div class="card-body">
        <span class="skeleton-line" style="width:30%;height:18px;"></span>
        <span class="skeleton-line" style="width:85%;height:20px;margin-top:8px;"></span>
        <span class="skeleton-line" style="width:60%;height:16px;"></span>
        <div class="card-meta" style="margin-top:14px;">
          <span class="skeleton-line" style="width:40%;height:28px;"></span>
          <span class="skeleton-line" style="width:45%;height:38px;border-radius:7px;"></span>
        </div>
      </div>`;
    grid.appendChild(ghost);
  }
  updateCount('…');
}

/** Блок ошибки подключения */
function showConnectionError() {
  grid.innerHTML = `
    <div class="api-error" role="alert">
      <div class="api-error-icon">⚠️</div>
      <h3 class="api-error-title">Ошибка подключения к серверу</h3>
      <p class="api-error-text">
        Не удалось загрузить каталог запчастей.<br>
        Убедитесь, что сервер запущен на <code>${API_PARTS}</code> и настроен CORS.
      </p>
      <button class="btn-reset" id="retryBtn">↻ Повторить попытку</button>
    </div>`;
  updateCount(0);
  document.getElementById('retryBtn')?.addEventListener('click', loadParts);
}

/**
 * Показывает статус под формой.
 * @param {string} text
 * @param {'ok'|'err'|''} type
 */
function setFormStatus(text, type = '') {
  formStatus.textContent  = text;
  formStatus.className    = `form-status ${type}`;
  if (text && type === 'ok') {
    // Автоматически скрываем успех через 3 сек
    setTimeout(() => setFormStatus(''), 3000);
  }
}

/** Добавить/убрать класс ошибки на поле ввода */
function markInput(el, hasError) {
  el.classList.toggle('input-error', hasError);
}

/* ─── 5. RENDER FUNCTIONS ──────────────────────────────────── */

function createCard(part, index) {
  const card = document.createElement('article');
  card.className = 'product-card';
  const icon = part.icon ?? FALLBACK_ICONS[index % FALLBACK_ICONS.length];

  card.innerHTML = `
    <div class="img-placeholder" aria-label="Изображение товара">${icon}</div>
    <div class="card-body">
      <span class="card-article"></span>
      <h3 class="card-name"></h3>
      <div class="card-meta">
        <span class="card-price"></span>
        <button class="btn-cart">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2.5">
            <circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/>
            <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/>
          </svg>
          В корзину
        </button>
      </div>
    </div>`;

  card.querySelector('.card-article').textContent = part.article ?? '—';
  card.querySelector('.card-name').textContent    = part.name    ?? 'Без названия';
  card.querySelector('.card-price').textContent   = formatPrice(part.price ?? 0);
  const btn = card.querySelector('.btn-cart');
  btn.dataset.article = part.article ?? '';
  btn.dataset.id      = part.id      ?? '';

  return card;
}

function renderGrid(parts) {
  grid.innerHTML = '';
  if (!parts || parts.length === 0) {
    noResults.classList.remove('hidden');
    updateCount(0);
    return;
  }
  noResults.classList.add('hidden');
  parts.forEach((part, i) => grid.appendChild(createCard(part, i)));
  updateCount(parts.length);
}

/* ─── 6. API — ЗАГРУЗКА ─────────────────────────────────────── */

/**
 * GET /api/parts
 * Загружает список запчастей с сервера и рисует каталог.
 */
async function loadParts() {
  showSkeleton();
  try {
    const res = await fetch(API_PARTS, {
      method:  'GET',
      headers: { 'Accept': 'application/json' }
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);

    const data = await res.json();
    if (!Array.isArray(data)) throw new Error('Ожидался массив от сервера.');

    serverParts = data;
    renderGrid(serverParts);
  } catch (err) {
    console.error('[AutoPartStore] Ошибка загрузки каталога:', err);
    showConnectionError();
  }
}

/* ─── 7. API — ДОБАВЛЕНИЕ ТОВАРА ────────────────────────────── */

/**
 * POST /api/parts/add
 * Читает поля формы, валидирует, отправляет новый товар на сервер.
 * После успеха (201) обновляет каталог через loadParts().
 */
async function addNewPart() {
  // Сбрасываем предыдущие ошибки
  [newName, newArticle, newPrice].forEach(el => markInput(el, false));
  setFormStatus('');

  const name    = newName.value.trim();
  const article = newArticle.value.trim();
  const price   = parseInt(newPrice.value, 10);

  // Валидация
  let hasError = false;
  if (!name)              { markInput(newName,    true); hasError = true; }
  if (!article)           { markInput(newArticle, true); hasError = true; }
  if (!newPrice.value || isNaN(price) || price < 0) {
    markInput(newPrice, true);
    hasError = true;
  }
  if (hasError) {
    setFormStatus('Заполните все поля корректно.', 'err');
    return;
  }

  // Блокируем кнопку на время запроса
  addBtn.disabled  = true;
  addBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none"
    stroke="currentColor" stroke-width="2.5" style="animation:spin .7s linear infinite">
    <path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg> Сохранение…`;

  try {
    const res = await fetch(API_ADD, {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify({ name, article, price })
    });

    if (res.status !== 201) {
      const msg = await res.text().catch(() => '');
      throw new Error(`Сервер вернул ${res.status}${msg ? ': ' + msg : ''}`);
    }

    // Успех — очищаем форму и перезагружаем каталог
    newName.value    = '';
    newArticle.value = '';
    newPrice.value   = '';
    setFormStatus(`✓ Товар «${name}» добавлен в каталог.`, 'ok');

    // Прокручиваем к каталогу и обновляем список
    document.getElementById('catalog')
      ?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    await loadParts();

  } catch (err) {
    console.error('[AutoPartStore] Ошибка добавления товара:', err);
    setFormStatus(`Ошибка: ${err.message}`, 'err');
  } finally {
    addBtn.disabled  = false;
    addBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none"
      stroke="currentColor" stroke-width="2.5">
      <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
    </svg> Сохранить в базу`;
  }
}

/* ─── 8. ПОИСК ──────────────────────────────────────────────── */

function handleSearch() {
  const query = searchInput.value.trim().toLowerCase();
  if (!query) { renderGrid(serverParts); return; }
  renderGrid(serverParts.filter(p =>
    (p.name    ?? '').toLowerCase().includes(query) ||
    (p.article ?? '').toLowerCase().includes(query)
  ));
}

/* ─── 9. КОРЗИНА ────────────────────────────────────────────── */

grid.addEventListener('click', e => {
  const btn = e.target.closest('.btn-cart');
  if (!btn) return;
  btn.classList.add('added');
  btn.disabled = true;
  btn.innerHTML = `
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
         stroke="currentColor" stroke-width="2.5">
      <polyline points="20 6 9 17 4 12"/>
    </svg> Добавлено!`;
});

/* ─── 10. EVENT LISTENERS ───────────────────────────────────── */

addBtn.addEventListener('click', addNewPart);

searchBtn.addEventListener('click', handleSearch);
searchInput.addEventListener('keydown', e => { if (e.key === 'Enter') handleSearch(); });
searchInput.addEventListener('input', handleSearch); // живой поиск при каждом вводе

resetBtn.addEventListener('click', () => {
  searchInput.value = '';
  renderGrid(serverParts);
});

quickTags.forEach(tag => {
  tag.addEventListener('click', () => {
    searchInput.value = tag.dataset.q;
    handleSearch();
    document.getElementById('catalog')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  });
});

burger.addEventListener('click', () => {
  burger.classList.toggle('open');
  mobileNav.classList.toggle('open');
});
mobileNav.querySelectorAll('.mobile-nav-link').forEach(link => {
  link.addEventListener('click', () => {
    burger.classList.remove('open');
    mobileNav.classList.remove('open');
  });
});

/* ─── 11. INIT ──────────────────────────────────────────────── */

loadParts();