const app = document.getElementById("app");

const statusClass = (s) => {
  if (s === "delivered") return "ok";
  if (s === "payment_failed" || s === "delivery_failed") return "bad";
  if (s === "out_of_stock" || s === "delivering" || s === "paid") return "warn";
  return "";
};

const money = (n) => new Intl.NumberFormat("ru-RU").format(n) + " ₽";

async function api(path, opts) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.message || data.error || res.statusText);
  return data;
}

function route() {
  const hash = location.hash.replace(/^#/, "") || "/";
  const parts = hash.split("/").filter(Boolean);
  if (parts[0] === "product" && parts[1]) return renderProduct(parts[1]);
  if (parts[0] === "order" && parts[1]) return renderOrder(parts[1]);
  if (parts[0] === "desk") return renderDesk();
  return renderCatalog();
}

async function renderCatalog() {
  app.innerHTML = `<div class="hero">
    <h1>Магазин кодов<br>без кассы и очередей</h1>
    <p>Ключи, пополнения и подписки. Оплата эмулируется вебхуком, выдача идёт сразу после подтверждения.</p>
  </div><div class="grid" id="grid">Загрузка…</div>`;
  const data = await api("/api/catalog?featured=1");
  document.getElementById("grid").innerHTML = data.products.map((p) => `
    <a class="card" href="#/product/${p.sku}">
      <img src="${p.image}" alt="" />
      <div class="type">${p.type}</div>
      <h3>${p.name}</h3>
      <div class="meta"><span>${money(p.price)}</span><span>остаток ${p.stock}</span></div>
    </a>
  `).join("");
}

async function renderProduct(sku) {
  const p = await api("/api/catalog/" + encodeURIComponent(sku));
  app.innerHTML = `<div class="panel">
    <img src="${p.image}" width="64" height="64" alt="" />
    <div class="type pill">${p.type}</div>
    <h1>${p.name}</h1>
    <div class="price">${money(p.price)}</div>
    <div class="meta">SKU ${p.sku} · в наличии ${p.stock}</div>
    <div class="row">
      <button id="buy">Оформить</button>
      <a class="btn ghost" href="#/">Назад</a>
    </div>
    <div id="msg" class="err"></div>
  </div>`;
  document.getElementById("buy").onclick = async () => {
    try {
      const order = await api("/api/orders", { method: "POST", body: JSON.stringify({ sku: p.sku }) });
      location.hash = "#/order/" + order.id;
    } catch (e) {
      document.getElementById("msg").textContent = e.message;
    }
  };
}

async function renderOrder(id) {
  const draw = (o) => {
    const code = o.delivery_code
      ? `<div class="keybox">${o.delivery_code}</div>`
      : `<p>Ключ появится здесь после оплаты и выдачи.</p>`;
    app.innerHTML = `<div class="panel">
      <div class="row">
        <span class="pill ${statusClass(o.status)}">${o.status}</span>
        <span class="pill">${o.id}</span>
      </div>
      <h1>${o.sku}</h1>
      <div class="price">${money(o.amount)}</div>
      ${code}
      <div class="row">
        ${o.status === "created" ? `<button id="pay">Оплатить</button>` : ""}
        <a class="btn ghost" href="#/">В каталог</a>
      </div>
      <div id="msg" class="err"></div>
    </div>`;
    const pay = document.getElementById("pay");
    if (pay) {
      pay.onclick = async () => {
        try {
          await api("/api/orders/" + o.id + "/pay", { method: "POST", body: "{}" });
          poll();
        } catch (e) {
          document.getElementById("msg").textContent = e.message;
        }
      };
    }
  };

  const poll = async () => {
    const o = await api("/api/orders/" + encodeURIComponent(id));
    draw(o);
    if (["delivered", "payment_failed"].includes(o.status)) return;
    if (["paid", "delivering", "out_of_stock", "delivery_failed"].includes(o.status)) {
      setTimeout(poll, 800);
    }
  };
  await poll();
}

async function renderDesk() {
  const r = await api("/admin/reconcile");
  const rows = (list) => list.map((o) => `<tr><td>${o.id}</td><td>${o.sku}</td><td>${o.status}</td><td>${money(o.amount)}</td></tr>`).join("") || `<tr><td colspan="4">пусто</td></tr>`;
  app.innerHTML = `<div class="panel">
    <h1>Сверка</h1>
    <p>Журнал: дебет ${r.ledger.debit} · кредит ${r.ledger.credit} · ${r.ledger.balanced ? "сходится" : "расхождение"}</p>
    <h3>Оплачен, не выдан</h3>
    <table><thead><tr><th>заказ</th><th>sku</th><th>статус</th><th>сумма</th></tr></thead><tbody>${rows(r.paid_not_delivered)}</tbody></table>
    <h3>Выдан, не оплачен</h3>
    <table><thead><tr><th>заказ</th><th>sku</th><th>статус</th><th>сумма</th></tr></thead><tbody>${rows(r.delivered_not_paid)}</tbody></table>
  </div>`;
}

window.addEventListener("hashchange", () => route().catch((e) => { app.innerHTML = `<p class="err">${e.message}</p>`; }));
route().catch((e) => { app.innerHTML = `<p class="err">${e.message}</p>`; });
