/* Mallard Server Hub — tiny syntax highlighter for curriculum & snippet docs
   Dependency-free; colors are wired to the hub palette in hub.css. */
(() => {
  "use strict";

  // All patterns must match at the current scan position, so they are
  // always anchored to the start.
  const read = (src, flags) => new RegExp("^" + src.replace(/^\^+/, ""), flags);

  // Ordered naturally: comments win over strings, strings win over keywords.
  const LANGS = {
    go: {
      comment: [read("^\\/\\/[^\\n]*"), read("^\\/\\*[\\s\\S]*?\\*\\/")],
      string: [
        read('^"(?:\\\\.|[^"\\\\\\n])*"', ""), read('^`(?:\\\\.|[^`\\\\])*`', ""),
        read("'(?:\\\\.|[^'\\\\\\n])*'", ""),
      ],
      keyword:
        "break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var nil true false func range".split(
          " "
        ),
    },
    rust: {
      comment: [read("^\\/\\/[^\\n]*"), read("^\\/\\*[\\s\\S]*?\\*\\/")],
      string: [read('^"(?:\\\\.|[^"\\\\\\n])*"', ""), read("^'(?:\\\\.|[^'\\\\\\n])*'", "")],
      keyword:
        "as async await break const continue crate dyn else enum extern fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait type unsafe use where while true false".split(
          " "
        ),
    },
    ts: {
      comment: [read("^\\/\\/[^\\n]*"), read("^\\/\\*\\*[\\s\\S]*?\\*\\/"), read("^\\/\\*[\\s\\S]*?\\*\\/")],
      string: [
        read('^"(?:\\\\.|[^"\\\\\\n])*"', ""), read("'(?:\\\\.|[^'\\\\\\n])*'", ""),
        read("^`(?:\\\\.|[^`\\\\])*`", ""),
      ],
      keyword:
        "abstract any as async await boolean break case catch class const constructor continue declare default delete do else enum export extends false finally for from function get if implements import in infer instanceof interface is keyof let never new null number object of package private protected public readonly return satisfies set static string super switch symbol this throw true try type typeguard typeof undefined unique unknown var void while with yield".split(
          " "
        ),
    },
    bash: {
      comment: [read("^#[^!][^\\n]*"), read("^#\\s[^\\n]*")],
      string: [
        read('^"(?:\\\\.|[^"\\\\])*"', ""), read("'[^']*'", ""),
        read("^\\$(\\w+|\\{[^}]*\\}|\\([^)]*\\))", ""),
      ],
      keyword:
        "if then else elif fi for while until do done case esac function return export source alias set unset local echo cd sudo apt install docker exec exit read printf shift trap eval".split(
          " "
        ),
    },
    sql: {
      comment: [read("^--[^\n]*"), read("^\\/\\*[\\s\\S]*?\\*\\/")],
      string: [
        read("'(?:''|[^'])*'", ""), read('^"(?:[^"])*"', ""), read("^`(?:[^`])*`", ""),
      ],
      keyword:
        "select from where insert into values update set delete create alter drop table database schema index view primary key foreign references constraint unique check default null not and or in like between is as order by group having limit offset join left right inner outer cross full on with recursive union all distinct case when then end exists analyze explain vacuum begin commit rollback grant revoke role user password extension trigger function returns language using serial uuid int integer bigint text varchar boolean timestamp timestamptz jsonb json numeric partition".split(
          " "
        ),
    },
    yaml: {
      comment: [read("^#[^\\n]*")],
      string: [read('^"(?:\\\\.|[^"\\\\\\n])*"', ""), read("'[^\\n']*'", "")],
      keyword: "true false null yes no on off".split(" "),
    },
  };
  LANGS.shell = LANGS.bash;
  LANGS.javascript = LANGS.ts;
  LANGS.js = LANGS.ts;
  LANGS.typescript = LANGS.ts;
  LANGS.json = LANGS.yaml; // strings + bare scalars only
  LANGS.dockerfile = {
    comment: [read("^#[^\n]*")],
    string: [read('^(?:"(?:\\\\.|[^"\\\\])*")', "")],
    keyword: "FROM RUN COPY ADD ENV ARG WORKDIR USER ENTRYPOINT CMD VOLUME EXPOSE LABEL HEALTHCHECK ONBUILD STOPSIGNAL SHELL as ALIAS".split(" "),
  };

  const FALLBACK = {
    comment: [read("^\\/\\/[^\\n]*"), read("^#[^\\n]*"), read("^\\/\\*[\\s\\S]*?\\*\\/"), read("^--[^\n]*")],
    string: [
      read('^"(?:\\\\.|[^"\\\\\\n])*"', ""), read("'(?:\\\\.|[^'\\\\\\n])*'", ""),
      read("^`(?:\\\\.|[^`\\\\])*`", ""),
    ],
    keyword: "",
  };

  const esc = (s) =>
    s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

  const wrap = (cls, text) => `<span class="tk-${cls}">${esc(text)}</span>`;

  // highlight turns raw code text into highlighted HTML.
  function highlight(code, lang) {
    const def = LANGS[lang] || FALLBACK;
    const kws = new Set(def.keyword || []);

    let out = "";
    let i = 0;

    const tryRe = (res) => {
      for (const re of res) {
        re.lastIndex = 0;
        const m = re.exec(code.slice(i));
        if (m) return m;
      }
      return null;
    };

    while (i < code.length) {
      let m = tryRe(def.comment);
      if (m) {
        out += wrap("comment", m[0]);
        i += m[0].length;
        continue;
      }
      m = tryRe(def.string);
      if (m) {
        out += wrap("string", m[0]);
        i += m[0].length;
        continue;
      }

      // numbers (decimal + hex)
      m = /^(?:0x[0-9a-fA-F]+|\d[\d_.]*)/.exec(code.slice(i));
      if (m) {
        out += wrap("number", m[0]);
        i += m[0].length;
        continue;
      }

      // identifiers (keyword sets are stored lowercase)
      m = /^[A-Za-z_@#$][\w@#$]*/.exec(code.slice(i));
      if (m) {
        const word = m[0];
        if (kws.has(word) || kws.has(word.toLowerCase())) {
          out += wrap("keyword", word);
        } else {
          out += esc(word);
        }
        i += word.length;
        continue;
      }

      const ch = code[i];
      out += esc(ch);
      i += 1;
    }
    return out;
  }

  // apply highlights every doc-code block on the page.
  window.hubHighlight = () => {
    document.querySelectorAll(".doc-code").forEach((fig) => {
      const code = fig.querySelector("code");
      if (!code || code.dataset.hl === "1") return;
      const lang = (fig.dataset.lang || "").toLowerCase();
      code.dataset.hl = "1";
      code.innerHTML = highlight(code.textContent, lang);
    });
  };
})();
