/* ============================================================
   AutoPartStore — script.js
   ============================================================ */

'use strict';

/* ─── 1. MOCK DATA ─────────────────────────────────────────── */
const mockParts = [
  {
    article: 'OC90',
    name:    'Масляный фильтр MANN OC90',
    price:   890,
    icon:    '🔧'
  },
  {
    article: 'BRK-44',
    name:    'Тормозные колодки Bosch передние',
    price:   3_250,
    icon:    '⚙️'
  },
  {
    article: 'SF-201',
    name:    'Топливный фильтр Filtron PP905',
    price:   1_140,
    icon:    '⛽'
  },
  {
    article: 'WT-8',
    name:    'Термостат охлаждающей системы',
    price:   2_600,
    icon:    '🌡️'
  },
  {
    article: 'SP-3320',
    name:    'Свечи зажигания NGK Iridium (к-т 4 шт.)',
    price:   4_800,
    icon:    '⚡'
  },
  {
    article: 'AB-77',
    name:    'Воздушный фильтр салона угольный',
    price:   760,
    icon:    '💨'
  },
  {
    article: 'TM-115',
    name:    'Ремень ГРМ Gates комплект с роликом',
    price:   5_490,
    icon:    '🔩'
  },
  {
    article: 'SH-202',
    name:    'Стойка переднего амортизатора Kayaba',
    price:   7_200,
    icon:    '🚗'
  }
];

/* ─── 2. DOM REFS ───────────────────────────────────────────── */
const grid         = document.getElementById('productGrid');
const searchInput  = document.getElementById('searchInput');
const searchBtn    = document.getElementById('searchBtn');
const noResults    = document.getElementById('noResults');
const resetBtn     = document.getElementById('resetBtn');
const resultsCount = document.getElementById('resultsCount');
const burger       = document.getElementById('burger');
const mobileNav    = document.getElementById('mobileNav');
const quickTags    = document.querySelectorAll('.quick-tag');

/* ─── 3. RENDER FUNCTION ───────────────────────────────────── */

/**
 * Форматирует число в цену: 3250 → «3 250 ₽»
 * @param {number} n
 * @returns {string}
 */
function formatPrice(n) {
  return n.toLocaleString('ru-RU') + ' ₽';
}

/**
 * Создаёт DOM-элемент карточки товара.
 * @param {object} part — объект из mockParts
 * @returns {HTMLElement}
 */
function createCard(part) {
  const card = document.createElement('article');
  card.className = 'product-card';

  card.innerHTML = `
    <div class="img-placeholder" aria-label="Изображение товара">
      ${part.icon}
    </div>
    <div class="card-body">
      <span class="card-article">${part.article}</span>
      <h3 class="card-name">${part.name}</h3>
      <div class="card-meta">
        <span class="card-price">${formatPrice(part.price)}</span>
        <button class="btn-cart" data-article="${part.article}">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2.5">
            <circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/>
            <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/>
          </svg>
          В корзину
        </button>
      </div>
    </div>
  `;

  return card;
}

/**
 * Отрисовывает массив товаров в сетке.
 * @param {object[]} parts
 */
function renderGrid(parts) {
  grid.innerHTML = '';

  if (parts.length === 0) {
    noResults.classList.remove('hidden');
    updateCount(0);
    return;
  }

  noResults.classList.add('hidden');

  parts.forEach(part => {
    grid.appendChild(createCard(part));
  });

  updateCount(parts.length);
}

/**
 * Обновляет счётчик найденных товаров.
 * @param {number} n
 */
function updateCount(n) {
  resultsCount.innerHTML = `Показано товаров: <strong>${n}</strong>`;
}

/* ─── 4. SEARCH LOGIC ──────────────────────────────────────── */

/**
 * Фильтрует mockParts по артикулу (регистронезависимо).
 */
function handleSearch() {
  const query = searchInput.value.trim().toUpperCase();

  if (!query) {
    renderGrid(mockParts);
    return;
  }

  const filtered = mockParts.filter(p =>
    p.article.toUpperCase().includes(query)
  );

  renderGrid(filtered);
}

/* ─── 5. CART BUTTON FEEDBACK ──────────────────────────────── */

/**
 * Обработчик клика «В корзину» — делегирование через grid.
 */
grid.addEventListener('click', e => {
  const btn = e.target.closest('.btn-cart');
  if (!btn) return;

  btn.classList.add('added');
  btn.innerHTML = `
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
         stroke="currentColor" stroke-width="2.5">
      <polyline points="20 6 9 17 4 12"/>
    </svg>
    Добавлено!
  `;
  btn.disabled = true;
});

/* ─── 6. EVENT LISTENERS ───────────────────────────────────── */

// Кнопка «Найти»
searchBtn.addEventListener('click', handleSearch);

// Enter в поле поиска
searchInput.addEventListener('keydown', e => {
  if (e.key === 'Enter') handleSearch();
});

// Быстрый сброс при очистке поля
searchInput.addEventListener('input', () => {
  if (searchInput.value.trim() === '') renderGrid(mockParts);
});

// Кнопка «Сбросить поиск»
resetBtn.addEventListener('click', () => {
  searchInput.value = '';
  renderGrid(mockParts);
});

// Быстрые теги артикулов
quickTags.forEach(tag => {
  tag.addEventListener('click', () => {
    searchInput.value = tag.dataset.q;
    handleSearch();
    document.getElementById('catalog')
      .scrollIntoView({ behavior: 'smooth', block: 'start' });
  });
});

// Бургер-меню
burger.addEventListener('click', () => {
  burger.classList.toggle('open');
  mobileNav.classList.toggle('open');
});

// Закрывать мобильное меню при клике на ссылку
mobileNav.querySelectorAll('.mobile-nav-link').forEach(link => {
  link.addEventListener('click', () => {
    burger.classList.remove('open');
    mobileNav.classList.remove('open');
  });
});

/* ─── 7. INIT ───────────────────────────────────────────────── */
renderGrid(mockParts);