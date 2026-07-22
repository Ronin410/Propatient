// Genera public/sitemap.xml antes del build, incluyendo el perfil público
// real de cada doctor listado (/dr/:slug) — antes solo tenía las 4 páginas
// estáticas (inicio, directorio, privacidad, términos) y dejaba que Google
// "descubriera solo" los perfiles de doctores siguiendo enlaces desde
// /doctores, lo cual es lento en un dominio nuevo con poco crawl budget.
//
// Se corre en cada `npm run build` (ver package.json), incluido el build
// de Render en cada deploy, así que el sitemap se mantiene al día con
// quién está listado públicamente sin depender de un paso manual.
//
// Si la API no responde (sin red en un entorno de build aislado, API caída
// un momento), no se rompe el build: se cae a las 4 páginas estáticas de
// siempre, igual que antes de este cambio.
import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const API_URL = process.env.VITE_API_URL || 'https://api.propatient.pro/api';
const SITE_URL = 'https://propatient.pro';
const OUT_PATH = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'public', 'sitemap.xml');

const STATIC_URLS = [
  { loc: `${SITE_URL}/`, changefreq: 'weekly', priority: '1.0' },
  { loc: `${SITE_URL}/doctores`, changefreq: 'daily', priority: '0.9' },
  { loc: `${SITE_URL}/privacidad`, changefreq: 'monthly', priority: '0.3' },
  { loc: `${SITE_URL}/terminos`, changefreq: 'monthly', priority: '0.3' },
];

async function fetchDoctorSlugs() {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 8000);
  try {
    const res = await fetch(`${API_URL}/public/doctors`, { signal: controller.signal });
    if (!res.ok) throw new Error(`API respondió ${res.status}`);
    const doctors = await res.json();
    return doctors
      .map((d) => d.publicSlug)
      .filter((slug) => typeof slug === 'string' && slug.length > 0);
  } finally {
    clearTimeout(timeout);
  }
}

function buildXml(urls) {
  const entries = urls.map(({ loc, changefreq, priority }) => `  <url>
    <loc>${loc}</loc>
    <changefreq>${changefreq}</changefreq>
    <priority>${priority}</priority>
  </url>`).join('\n');

  return `<?xml version="1.0" encoding="UTF-8"?>
<!--
  Generado automáticamente por scripts/generate-sitemap.mjs en cada build
  (no editar a mano — los cambios se pierden en el siguiente build).
-->
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${entries}
</urlset>
`;
}

let slugs = [];
try {
  slugs = await fetchDoctorSlugs();
  console.log(`sitemap: ${slugs.length} perfil(es) de doctor incluido(s)`);
} catch (err) {
  console.warn(`sitemap: no se pudo obtener la lista de doctores (${err.message}); se genera solo con las páginas estáticas`);
}

const doctorUrls = slugs.map((slug) => ({
  loc: `${SITE_URL}/dr/${slug}`,
  changefreq: 'weekly',
  priority: '0.7',
}));

writeFileSync(OUT_PATH, buildXml([...STATIC_URLS, ...doctorUrls]));
console.log(`sitemap.xml escrito en ${OUT_PATH}`);
