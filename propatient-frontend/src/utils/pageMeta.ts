// Actualiza el <title> y los meta tags de descripción/Open Graph desde una
// página pública específica (directorio, perfil de un doctor). Sin esto,
// TODAS las rutas comparten el mismo <title>/<meta description> estáticos
// de index.html (es una SPA, un solo HTML para todas las rutas) — Google
// no puede distinguir el perfil de un doctor del de otro ni de la portada,
// así que en resultados de búsqueda cae de vuelta a fragmentos de texto
// tomados al vuelo de la página.
//
// Limitación real: esto solo ayuda a rastreadores que ejecutan JavaScript
// (Googlebot sí lo hace, aunque con una segunda pasada más lenta que la
// portada estática). Vistas previas de enlaces que NO ejecutan JS (algunos
// bots de WhatsApp/Facebook más antiguos) seguirán viendo el título/
// descripción genéricos de index.html — arreglar eso de raíz requeriría
// renderizado en servidor, fuera del alcance de este cambio.
//
// Devuelve una función para restaurar los valores previos — se llama en el
// cleanup del useEffect que invoque a setPageMeta, para no dejar el título
// de un doctor pegado si el usuario navega a otra pantalla sin su propio
// setPageMeta.
export function setPageMeta({ title, description }: { title: string; description: string }): () => void {
  const previousTitle = document.title;
  document.title = title;

  const restoreFns: Array<() => void> = [];

  const setMeta = (selector: string, value: string) => {
    const el = document.querySelector<HTMLMetaElement>(selector);
    if (!el) return;
    const previous = el.getAttribute('content') ?? '';
    el.setAttribute('content', value);
    restoreFns.push(() => el.setAttribute('content', previous));
  };

  setMeta('meta[name="description"]', description);
  setMeta('meta[property="og:title"]', title);
  setMeta('meta[property="og:description"]', description);
  setMeta('meta[name="twitter:title"]', title);
  setMeta('meta[name="twitter:description"]', description);

  return () => {
    document.title = previousTitle;
    restoreFns.forEach((restore) => restore());
  };
}
