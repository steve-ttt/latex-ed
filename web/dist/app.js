// ==============================================================================
// LaTeX-Ed - Core Application Logic
// ==============================================================================

(function() {
  'use strict';

  // ----------------------------------------------------------------------------
  // 1. Application State & DOM Element References
  // ----------------------------------------------------------------------------
  let currentFile = '';
  let files = [];
  let isCompiling = false;
  let diagnostics = [];
  let folderState = {}; // path -> boolean (expanded)
  let wordWrap = true;

  const codeEditor = document.getElementById('codeEditor');
  const highlightLayer = document.getElementById('highlightLayer');
  const lineNumbers = document.getElementById('lineNumbers');
  const fileTree = document.getElementById('fileTree');
  const pdfFrame = document.getElementById('pdfFrame');
  const pdfEmpty = document.getElementById('pdfEmpty');
  const btnCompile = document.getElementById('btnCompile');
  const btnSave = document.getElementById('btnSave');
  const logPanel = document.getElementById('logPanel');
  const logBar = document.getElementById('logBar');
  const logBody = document.getElementById('logBody');
  const logToggle = document.getElementById('logToggle');
  const statErrors = document.getElementById('statErrors');
  const statWarnings = document.getElementById('statWarnings');
  const statDuration = document.getElementById('statDuration');
  const editorStatus = document.getElementById('editorStatus');
  const headerFile = document.getElementById('headerFile');
  const btnToggleWrap = document.getElementById('btnToggleWrap');
  const btnSyncTex = document.getElementById('btnSyncTex');
  const btnNewFile = document.getElementById('btnNewFile');
  const btnNewFolder = document.getElementById('btnNewFolder');
  const btnInitialCompile = document.getElementById('btnInitialCompile');
  const btnDownload = document.getElementById('btnDownload');
  const btnOpenNew = document.getElementById('btnOpenNew');

  // Insert Snippet Elements
  const btnInsert = document.getElementById('btnInsert');
  const insertPalette = document.getElementById('insertPalette');
  const insertSearch = document.getElementById('insertSearch');
  const insertClose = document.getElementById('insertClose');
  const insertTabs = document.getElementById('insertTabs');
  const insertList = document.getElementById('insertList');

  // Find & Replace Elements & State
  const btnFind = document.getElementById('btnFind');
  const findWidget = document.getElementById('findWidget');
  const findInput = document.getElementById('findInput');
  const replaceInput = document.getElementById('replaceInput');
  const replaceRow = document.getElementById('replaceRow');
  const findToggleReplace = document.getElementById('findToggleReplace');
  const findCaseSensitive = document.getElementById('findCaseSensitive');
  const findPrev = document.getElementById('findPrev');
  const findNext = document.getElementById('findNext');
  const findClose = document.getElementById('findClose');
  const findMatchesCount = document.getElementById('findMatchesCount');
  const btnReplaceOne = document.getElementById('btnReplaceOne');
  const btnReplaceAll = document.getElementById('btnReplaceAll');

  let findMatches = [];
  let currentMatchIndex = -1;
  let matchCase = false;

  // ----------------------------------------------------------------------------
  // 2. HTML Escaping Utility
  // ----------------------------------------------------------------------------
  function escapeHtml(str) {
    if (!str) return '';
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  // ----------------------------------------------------------------------------
  // 3. LaTeX Syntax Highlighting Tokenizer
  // ----------------------------------------------------------------------------
  function highlightLatex(text) {
    if (!text) return ' ';
    const src = text;
    const endsWithNewline = src.endsWith('\n');

    const parts = [];
    let lastIndex = 0;

    // Tokens:
    // 1. Comments: %[^\n]*
    // 2. Environments: \(begin|end){env}
    // 3. Math delimiters: $$, $, \[, \], \(, \)
    // 4. Commands / escaped chars: \\[a-zA-Z@]+ or \\.
    // 5. Braces & Brackets: [{}[\]]
    // 6. Alignment & table separators: &
    // 7. Numbers & measurements: digits with optional units
    const tokenRegex = /(%[^\n]*)|(\\(?:begin|end)\{([a-zA-Z0-9*_\-]+)\})|(\$\$?|\\\[|\\\]|\\\(|\\\))|(\\[a-zA-Z@]+|\\.)|([{}[\]])|(&)|(\b\d+(?:\.\d+)?(?:pt|em|ex|cm|mm|in)?\b)/g;

    let match;
    while ((match = tokenRegex.exec(src)) !== null) {
      if (match.index > lastIndex) {
        parts.push(escapeHtml(src.substring(lastIndex, match.index)));
      }
      const [full, comment, envFull, envName, math, cmd, brace, align, num] = match;

      if (comment) {
        parts.push('<span class="hl-comment">' + escapeHtml(comment) + '</span>');
      } else if (envFull) {
        const isBegin = envFull.startsWith('\\begin');
        const kw = isBegin ? '\\begin' : '\\end';
        parts.push('<span class="hl-command">' + kw + '</span><span class="hl-bracket">{</span><span class="hl-env">' + escapeHtml(envName) + '</span><span class="hl-bracket">}</span>');
      } else if (math) {
        parts.push('<span class="hl-math">' + escapeHtml(math) + '</span>');
      } else if (cmd) {
        parts.push('<span class="hl-command">' + escapeHtml(cmd) + '</span>');
      } else if (brace) {
        parts.push('<span class="hl-bracket">' + escapeHtml(brace) + '</span>');
      } else if (align) {
        parts.push('<span class="hl-align">' + escapeHtml(align) + '</span>');
      } else if (num) {
        parts.push('<span class="hl-num">' + escapeHtml(num) + '</span>');
      }
      lastIndex = tokenRegex.lastIndex;
    }

    if (lastIndex < src.length) {
      parts.push(escapeHtml(src.substring(lastIndex)));
    }

    if (endsWithNewline) {
      parts.push('\n ');
    }

    return parts.join('');
  }

  function syncHighlightDimensions() {
    if (!highlightLayer || !codeEditor) return;
    const w = codeEditor.clientWidth;
    const h = codeEditor.clientHeight;
    if (w > 0) highlightLayer.style.width = w + 'px';
    if (h > 0) highlightLayer.style.height = h + 'px';
  }

  function updateHighlight() {
    if (!highlightLayer || !codeEditor) return;
    highlightLayer.innerHTML = highlightLatex(codeEditor.value);
    syncHighlightDimensions();
  }

  // ----------------------------------------------------------------------------
  // 4. Line Numbers Gutter & Hidden Height Measurement Mirror
  // ----------------------------------------------------------------------------
  const measureDiv = document.createElement('div');
  measureDiv.id = 'measureDiv';
  measureDiv.style.cssText = `
    position: absolute;
    top: -99999px;
    left: -99999px;
    visibility: hidden;
    height: auto;
    white-space: pre-wrap;
    word-break: break-word;
    overflow-wrap: break-word;
    font-family: 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace;
    font-size: 13px;
    line-height: 20px;
    box-sizing: border-box;
    padding: 8px 12px;
    border: none;
  `;
  document.body.appendChild(measureDiv);

  function updateLineNumbers() {
    const lines = codeEditor.value.split('\n');
    const curBase = currentFile.split('/').pop();
    const errLines = new Set(
      diagnostics
        .filter(d => (!d.file || d.file === currentFile || d.file.endsWith(curBase)) && d.severity === 'error')
        .map(d => d.line)
    );

    if (!wordWrap) {
      codeEditor.style.whiteSpace = 'pre';
      codeEditor.style.overflowWrap = 'normal';
      codeEditor.style.wordBreak = 'normal';
      codeEditor.style.overflowX = 'auto';

      highlightLayer.style.whiteSpace = 'pre';
      highlightLayer.style.overflowWrap = 'normal';
      highlightLayer.style.wordBreak = 'normal';
      highlightLayer.style.overflowX = 'auto';

      let nums = '';
      for (let i = 1; i <= lines.length; i++) {
        const isErr = errLines.has(i);
        nums += `<div class="${isErr ? 'err-line' : ''}" style="height:20px;line-height:20px;">${i}</div>`;
      }
      lineNumbers.innerHTML = nums;
      syncHighlightDimensions();
      return;
    }

    // Word Wrap is ON
    codeEditor.style.whiteSpace = 'pre-wrap';
    codeEditor.style.overflowWrap = 'break-word';
    codeEditor.style.wordBreak = 'break-word';
    codeEditor.style.overflowX = 'hidden';

    highlightLayer.style.whiteSpace = 'pre-wrap';
    highlightLayer.style.overflowWrap = 'break-word';
    highlightLayer.style.wordBreak = 'break-word';
    highlightLayer.style.overflowX = 'hidden';

    const width = codeEditor.clientWidth;
    if (width > 0) {
      measureDiv.style.width = width + 'px';
    }

    measureDiv.innerHTML = lines.map(line => `<div>${escapeHtml(line) || '&nbsp;'}</div>`).join('');

    const children = measureDiv.children;
    let nums = '';
    for (let i = 0; i < lines.length; i++) {
      const lineNum = i + 1;
      const h = children[i] ? children[i].getBoundingClientRect().height : 20;
      const isErr = errLines.has(lineNum);
      nums += `<div class="${isErr ? 'err-line' : ''}" style="height:${h}px;line-height:20px;">${lineNum}</div>`;
    }
    lineNumbers.innerHTML = nums;
    syncHighlightDimensions();
  }

  btnToggleWrap.onclick = () => {
    wordWrap = !wordWrap;
    btnToggleWrap.textContent = wordWrap ? '↩ Wrap: On' : '↩ Wrap: Off';
    btnToggleWrap.style.color = wordWrap ? 'var(--text-bright)' : 'var(--text-muted)';
    updateLineNumbers();
    updateHighlight();
  };

  if (window.ResizeObserver) {
    new ResizeObserver(() => {
      syncHighlightDimensions();
      if (wordWrap) {
        updateLineNumbers();
      }
    }).observe(codeEditor);
  }

  codeEditor.addEventListener('scroll', () => {
    highlightLayer.scrollTop = codeEditor.scrollTop;
    highlightLayer.scrollLeft = codeEditor.scrollLeft;
    lineNumbers.scrollTop = codeEditor.scrollTop;
  });

  codeEditor.addEventListener('input', () => {
    updateHighlight();
    updateLineNumbers();
    editorStatus.textContent = 'Unsaved changes';
  });

  codeEditor.addEventListener('keydown', (e) => {
    if (e.key === 'Tab') {
      e.preventDefault();
      const start = codeEditor.selectionStart;
      const end = codeEditor.selectionEnd;
      codeEditor.value = codeEditor.value.substring(0, start) + '  ' + codeEditor.value.substring(end);
      codeEditor.selectionStart = codeEditor.selectionEnd = start + 2;
      updateHighlight();
      updateLineNumbers();
    }
  });

  // ----------------------------------------------------------------------------
  // 5. Categorized LaTeX Snippets Palette
  // ----------------------------------------------------------------------------
  const SNIPPETS = [
    // Environments & Lists
    { cat: 'Environments', label: 'itemize (bulleted list)', badge: '•', code: '\\begin{itemize}', body: '\\begin{itemize}\n  \\item $0\n\\end{itemize}' },
    { cat: 'Environments', label: 'enumerate (numbered list)', badge: '1.', code: '\\begin{enumerate}', body: '\\begin{enumerate}\n  \\item $0\n\\end{enumerate}' },
    { cat: 'Environments', label: 'item (list item)', badge: '•', code: '\\item', body: '\\item $0' },
    { cat: 'Environments', label: 'description (term & definition)', badge: '≡', code: '\\begin{description}', body: '\\begin{description}\n  \\item[$0] \n\\end{description}' },
    { cat: 'Environments', label: 'equation (numbered equation)', badge: 'fx', code: '\\begin{equation}', body: '\\begin{equation}\n  $0\n\\end{equation}' },
    { cat: 'Environments', label: 'align* (aligned equations)', badge: '&=', code: '\\begin{align*}', body: '\\begin{align*}\n  $0 &= \n\\end{align*}' },
    { cat: 'Environments', label: 'figure (image & caption)', badge: '🖼️', code: '\\begin{figure}', body: '\\begin{figure}[htbp]\n  \\centering\n  \\includegraphics[width=0.8\\textwidth]{$0}\n  \\caption{Caption}\n  \\label{fig:label}\n\\end{figure}' },
    { cat: 'Environments', label: 'table (tabular grid)', badge: '▦', code: '\\begin{table}', body: '\\begin{table}[htbp]\n  \\centering\n  \\begin{tabular}{|c|c|}\n    \\hline\n    $0 &  \\\\\n    \\hline\n  \\end{tabular}\n  \\caption{Caption}\n  \\label{tab:label}\n\\end{table}' },
    { cat: 'Environments', label: 'lstlisting (code block)', badge: '💻', code: '\\begin{lstlisting}', body: '\\begin{lstlisting}\n$0\n\\end{lstlisting}' },
    { cat: 'Environments', label: 'lstlisting (with language)', badge: '💻', code: '\\begin{lstlisting}[language=...]', body: '\\begin{lstlisting}[language=$0]\n\n\\end{lstlisting}' },
    { cat: 'Environments', label: 'section (heading 1)', badge: 'H1', code: '\\section{}', wrapPrefix: '\\section{', wrapSuffix: '}' },
    { cat: 'Environments', label: 'subsection (heading 2)', badge: 'H2', code: '\\subsection{}', wrapPrefix: '\\subsection{', wrapSuffix: '}' },
    { cat: 'Environments', label: 'subsubsection (heading 3)', badge: 'H3', code: '\\subsubsection{}', wrapPrefix: '\\subsubsection{', wrapSuffix: '}' },

    // Text Formatting
    { cat: 'Text', label: 'bold (textbf)', badge: 'B', code: '\\textbf{}', wrapPrefix: '\\textbf{', wrapSuffix: '}' },
    { cat: 'Text', label: 'italic (textit)', badge: 'I', code: '\\textit{}', wrapPrefix: '\\textit{', wrapSuffix: '}' },
    { cat: 'Text', label: 'typewriter / code (texttt)', badge: 'TT', code: '\\texttt{}', wrapPrefix: '\\texttt{', wrapSuffix: '}' },
    { cat: 'Text', label: 'lstinline (inline code)', badge: '`', code: '\\lstinline||', wrapPrefix: '\\lstinline|', wrapSuffix: '|' },
    { cat: 'Text', label: 'underline (underline)', badge: 'U', code: '\\underline{}', wrapPrefix: '\\underline{', wrapSuffix: '}' },
    { cat: 'Text', label: 'emphasize (emph)', badge: 'Em', code: '\\emph{}', wrapPrefix: '\\emph{', wrapSuffix: '}' },
    { cat: 'Text', label: 'quote (block quotation)', badge: '“ ”', code: '\\begin{quote}', body: '\\begin{quote}\n  $0\n\\end{quote}' },
    { cat: 'Text', label: 'footnote (footnote text)', badge: '¹', code: '\\footnote{}', wrapPrefix: '\\footnote{', wrapSuffix: '}' },
    { cat: 'Text', label: 'cite (citation reference)', badge: '📖', code: '\\cite{}', wrapPrefix: '\\cite{', wrapSuffix: '}' },
    { cat: 'Text', label: 'ref (cross reference)', badge: '🔗', code: '\\ref{}', wrapPrefix: '\\ref{', wrapSuffix: '}' },

    // Math & Calculus
    { cat: 'Math', label: 'frac (fraction)', badge: 'a/b', code: '\\frac{num}{den}', wrapPrefix: '\\frac{', wrapSuffix: '}{}' },
    { cat: 'Math', label: 'sqrt (square root)', badge: '√', code: '\\sqrt{}', wrapPrefix: '\\sqrt{', wrapSuffix: '}' },
    { cat: 'Math', label: 'sum (summation with bounds)', badge: '∑', code: '\\sum_{i=1}^{n}', body: '\\sum_{i=1}^{n} $0' },
    { cat: 'Math', label: 'int (definite integral)', badge: '∫', code: '\\int_{a}^{b} ... dx', body: '\\int_{a}^{b} $0 \\, dx' },
    { cat: 'Math', label: 'int (indefinite integral)', badge: '∫', code: '\\int ... dx', body: '\\int $0 \\, dx' },
    { cat: 'Math', label: 'prod (product with bounds)', badge: '∏', code: '\\prod_{i=1}^{n}', body: '\\prod_{i=1}^{n} $0' },
    { cat: 'Math', label: 'lim (limit)', badge: 'lim', code: '\\lim_{x \\to \\infty}', body: '\\lim_{x \\to \\infty} $0' },
    { cat: 'Math', label: 'partial (derivative)', badge: '∂', code: '\\frac{\\partial f}{\\partial x}', body: '\\frac{\\partial $0}{\\partial x}' },
    { cat: 'Math', label: 'superscript (power)', badge: 'x²', code: '^{}', wrapPrefix: '^{', wrapSuffix: '}' },
    { cat: 'Math', label: 'subscript (index)', badge: 'x₁', code: '_{}', wrapPrefix: '_{', wrapSuffix: '}' },
    { cat: 'Math', label: 'infty (infinity)', badge: '∞', code: '\\infty', body: '\\infty' },
    { cat: 'Math', label: 'pm (plus-minus)', badge: '±', code: '\\pm', body: '\\pm' },
    { cat: 'Math', label: 'cdot (centered dot)', badge: '·', code: '\\cdot', body: '\\cdot' },
    { cat: 'Math', label: 'times (multiplication cross)', badge: '×', code: '\\times', body: '\\times' },

    // Greek Letters
    { cat: 'Greek', label: 'alpha', badge: 'α', code: '\\alpha', body: '\\alpha' },
    { cat: 'Greek', label: 'beta', badge: 'β', code: '\\beta', body: '\\beta' },
    { cat: 'Greek', label: 'gamma', badge: 'γ', code: '\\gamma', body: '\\gamma' },
    { cat: 'Greek', label: 'Gamma (capital)', badge: 'Γ', code: '\\Gamma', body: '\\Gamma' },
    { cat: 'Greek', label: 'delta', badge: 'δ', code: '\\delta', body: '\\delta' },
    { cat: 'Greek', label: 'Delta (capital)', badge: 'Δ', code: '\\Delta', body: '\\Delta' },
    { cat: 'Greek', label: 'epsilon', badge: 'ϵ', code: '\\epsilon', body: '\\epsilon' },
    { cat: 'Greek', label: 'zeta', badge: 'ζ', code: '\\zeta', body: '\\zeta' },
    { cat: 'Greek', label: 'eta', badge: 'η', code: '\\eta', body: '\\eta' },
    { cat: 'Greek', label: 'theta', badge: 'θ', code: '\\theta', body: '\\theta' },
    { cat: 'Greek', label: 'Theta (capital)', badge: 'Θ', code: '\\Theta', body: '\\Theta' },
    { cat: 'Greek', label: 'lambda', badge: 'λ', code: '\\lambda', body: '\\lambda' },
    { cat: 'Greek', label: 'Lambda (capital)', badge: 'Λ', code: '\\Lambda', body: '\\Lambda' },
    { cat: 'Greek', label: 'mu', badge: 'μ', code: '\\mu', body: '\\mu' },
    { cat: 'Greek', label: 'pi', badge: 'π', code: '\\pi', body: '\\pi' },
    { cat: 'Greek', label: 'Pi (capital)', badge: 'Π', code: '\\Pi', body: '\\Pi' },
    { cat: 'Greek', label: 'rho', badge: 'ρ', code: '\\rho', body: '\\rho' },
    { cat: 'Greek', label: 'sigma', badge: 'σ', code: '\\sigma', body: '\\sigma' },
    { cat: 'Greek', label: 'Sigma (capital)', badge: 'Σ', code: '\\Sigma', body: '\\Sigma' },
    { cat: 'Greek', label: 'tau', badge: 'τ', code: '\\tau', body: '\\tau' },
    { cat: 'Greek', label: 'phi', badge: 'ϕ', code: '\\phi', body: '\\phi' },
    { cat: 'Greek', label: 'Phi (capital)', badge: 'Φ', code: '\\Phi', body: '\\Phi' },
    { cat: 'Greek', label: 'psi', badge: 'ψ', code: '\\psi', body: '\\psi' },
    { cat: 'Greek', label: 'Psi (capital)', badge: 'Ψ', code: '\\Psi', body: '\\Psi' },
    { cat: 'Greek', label: 'omega', badge: 'ω', code: '\\omega', body: '\\omega' },
    { cat: 'Greek', label: 'Omega (capital)', badge: 'Ω', code: '\\Omega', body: '\\Omega' },

    // Relational & Logic
    { cat: 'Relational', label: 'leq (less or equal)', badge: '≤', code: '\\leq', body: '\\leq' },
    { cat: 'Relational', label: 'geq (greater or equal)', badge: '≥', code: '\\geq', body: '\\geq' },
    { cat: 'Relational', label: 'neq (not equal)', badge: '≠', code: '\\neq', body: '\\neq' },
    { cat: 'Relational', label: 'approx (approximately equal)', badge: '≈', code: '\\approx', body: '\\approx' },
    { cat: 'Relational', label: 'equiv (equivalent / identical)', badge: '≡', code: '\\equiv', body: '\\equiv' },
    { cat: 'Relational', label: 'sim (similar / proportional)', badge: '∼', code: '\\sim', body: '\\sim' },
    { cat: 'Relational', label: 'subset (proper subset)', badge: '⊂', code: '\\subset', body: '\\subset' },
    { cat: 'Relational', label: 'subseteq (subset or equal)', badge: '⊆', code: '\\subseteq', body: '\\subseteq' },
    { cat: 'Relational', label: 'supset (superset)', badge: '⊃', code: '\\supset', body: '\\supset' },
    { cat: 'Relational', label: 'in (element of)', badge: '∈', code: '\\in', body: '\\in' },
    { cat: 'Relational', label: 'notin (not element of)', badge: '∉', code: '\\notin', body: '\\notin' },
    { cat: 'Relational', label: 'forall (for all quantifier)', badge: '∀', code: '\\forall', body: '\\forall' },
    { cat: 'Relational', label: 'exists (there exists)', badge: '∃', code: '\\exists', body: '\\exists' },
    { cat: 'Relational', label: 'implies (long right arrow)', badge: '⟹', code: '\\implies', body: '\\implies' },
    { cat: 'Relational', label: 'iff (if and only if)', badge: '⟺', code: '\\iff', body: '\\iff' },
    { cat: 'Relational', label: 'to (right arrow)', badge: '→', code: '\\to', body: '\\to' },
    { cat: 'Relational', label: 'gets (left arrow)', badge: '←', code: '\\gets', body: '\\gets' },
    { cat: 'Relational', label: 'mapsto (maps to)', badge: '↦', code: '\\mapsto', body: '\\mapsto' },

    // Matrices & Arrays
    { cat: 'Matrices', label: 'pmatrix (parentheses matrix)', badge: '( )', code: '\\begin{pmatrix}', body: '\\begin{pmatrix}\n  $0 &  \\\\\n   & \n\\end{pmatrix}' },
    { cat: 'Matrices', label: 'bmatrix (bracket matrix)', badge: '[ ]', code: '\\begin{bmatrix}', body: '\\begin{bmatrix}\n  $0 &  \\\\\n   & \n\\end{bmatrix}' },
    { cat: 'Matrices', label: 'matrix (plain matrix)', badge: '▦', code: '\\begin{matrix}', body: '\\begin{matrix}\n  $0 &  \\\\\n   & \n\\end{matrix}' },
    { cat: 'Matrices', label: 'vmatrix (determinant / bars)', badge: '| |', code: '\\begin{vmatrix}', body: '\\begin{vmatrix}\n  $0 &  \\\\\n   & \n\\end{vmatrix}' },
    { cat: 'Matrices', label: 'cases (piecewise cases)', badge: '{ ', code: '\\begin{cases}', body: '\\begin{cases}\n  $0, & \\text{if }  \\\\\n  , & \\text{otherwise}\n\\end{cases}' }
  ];

  let currentCategory = 'all';
  let filteredSnippets = [];
  let selectedSnippetIndex = 0;

  function renderInsertList() {
    const q = insertSearch.value.trim().toLowerCase();
    filteredSnippets = SNIPPETS.filter(s => {
      const matchesCat = (currentCategory === 'all' || s.cat === currentCategory);
      if (!matchesCat) return false;
      if (!q) return true;
      return s.label.toLowerCase().includes(q) ||
             s.code.toLowerCase().includes(q) ||
             s.cat.toLowerCase().includes(q) ||
             (s.badge && s.badge.toLowerCase().includes(q));
    });

    if (filteredSnippets.length === 0) {
      insertList.innerHTML = '<div class="insert-empty">No matching LaTeX tags found</div>';
      selectedSnippetIndex = -1;
      return;
    }

    if (selectedSnippetIndex >= filteredSnippets.length || selectedSnippetIndex < 0) {
      selectedSnippetIndex = 0;
    }

    let html = '';
    let lastCat = '';
    filteredSnippets.forEach((item, idx) => {
      if (currentCategory === 'all' && item.cat !== lastCat) {
        lastCat = item.cat;
        html += `<div class="insert-category-header">${escapeHtml(lastCat)}</div>`;
      }
      const isSelected = (idx === selectedSnippetIndex);
      html += `
        <div class="insert-item ${isSelected ? 'selected' : ''}" data-idx="${idx}">
          <span class="insert-badge">${escapeHtml(item.badge || '')}</span>
          <span class="insert-name">${escapeHtml(item.label)}</span>
          <code class="insert-code">${escapeHtml(item.code)}</code>
        </div>
      `;
    });

    insertList.innerHTML = html;

    const items = insertList.querySelectorAll('.insert-item');
    items.forEach(el => {
      el.onclick = () => {
        const idx = parseInt(el.getAttribute('data-idx'), 10);
        if (filteredSnippets[idx]) {
          insertSnippet(filteredSnippets[idx]);
        }
      };
    });

    const selectedEl = insertList.querySelector('.insert-item.selected');
    if (selectedEl) {
      selectedEl.scrollIntoView({ block: 'nearest' });
    }
  }

  function insertSnippet(item) {
    const start = codeEditor.selectionStart;
    const end = codeEditor.selectionEnd;
    const val = codeEditor.value;
    const selection = val.substring(start, end);

    let insertedText = '';
    let targetCaret = start;

    if (item.wrapPrefix && item.wrapSuffix) {
      if (selection) {
        insertedText = item.wrapPrefix + selection + item.wrapSuffix;
        targetCaret = start + insertedText.length;
      } else {
        insertedText = item.wrapPrefix + item.wrapSuffix;
        targetCaret = start + item.wrapPrefix.length;
      }
    } else if (item.body) {
      const placeholderIdx = item.body.indexOf('$0');
      if (placeholderIdx !== -1) {
        const before = item.body.substring(0, placeholderIdx);
        const after = item.body.substring(placeholderIdx + 2);
        insertedText = before + selection + after;
        targetCaret = start + before.length + (selection ? selection.length : 0);
      } else {
        insertedText = item.body + ' ';
        targetCaret = start + insertedText.length;
      }
    } else if (item.code) {
      insertedText = item.code + ' ';
      targetCaret = start + insertedText.length;
    }

    codeEditor.value = val.substring(0, start) + insertedText + val.substring(end);
    codeEditor.focus();
    codeEditor.setSelectionRange(targetCaret, targetCaret);

    updateHighlight();
    updateLineNumbers();
    editorStatus.textContent = 'Unsaved changes';
    closeInsertPalette();
  }

  function openInsertPalette() {
    findWidget.classList.add('hidden');
    insertPalette.classList.remove('hidden');
    btnInsert.classList.add('primary');
    insertSearch.value = '';
    currentCategory = 'all';
    insertTabs.querySelectorAll('.insert-tab').forEach((t, i) => {
      t.classList.toggle('active', i === 0);
    });
    selectedSnippetIndex = 0;
    renderInsertList();
    setTimeout(() => insertSearch.focus(), 20);
  }

  function closeInsertPalette() {
    insertPalette.classList.add('hidden');
    btnInsert.classList.remove('primary');
    codeEditor.focus();
  }

  function toggleInsertPalette() {
    if (insertPalette.classList.contains('hidden')) {
      openInsertPalette();
    } else {
      closeInsertPalette();
    }
  }

  // ----------------------------------------------------------------------------
  // 6. Find & Replace Controller
  // ----------------------------------------------------------------------------
  function openFind(withReplace = false) {
    closeInsertPalette();
    findWidget.classList.remove('hidden');
    if (withReplace) {
      replaceRow.classList.remove('hidden');
      findToggleReplace.textContent = '▼';
    }

    const selStart = codeEditor.selectionStart;
    const selEnd = codeEditor.selectionEnd;
    if (selEnd > selStart) {
      const selected = codeEditor.value.substring(selStart, selEnd);
      if (selected && !selected.includes('\n')) {
        findInput.value = selected;
      }
    }

    if (withReplace && findInput.value) {
      replaceInput.focus();
      replaceInput.select();
    } else {
      findInput.focus();
      findInput.select();
    }
    performFind();
  }

  function closeFind() {
    findWidget.classList.add('hidden');
    codeEditor.focus();
  }

  function performFind() {
    const query = findInput.value;
    if (!query) {
      findMatches = [];
      currentMatchIndex = -1;
      findMatchesCount.textContent = 'No results';
      return;
    }

    const text = codeEditor.value;
    const flags = matchCase ? 'g' : 'gi';
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(escaped, flags);

    findMatches = [];
    let match;
    while ((match = regex.exec(text)) !== null) {
      findMatches.push({ start: match.index, end: match.index + match[0].length });
    }

    if (findMatches.length === 0) {
      currentMatchIndex = -1;
      findMatchesCount.textContent = 'No results';
      return;
    }

    const selStart = codeEditor.selectionStart;
    currentMatchIndex = findMatches.findIndex(m => m.start >= selStart);
    if (currentMatchIndex === -1) {
      currentMatchIndex = 0;
    }

    highlightMatch(currentMatchIndex);
  }

  function highlightMatch(idx) {
    if (idx < 0 || idx >= findMatches.length) return;
    currentMatchIndex = idx;
    findMatchesCount.textContent = `${currentMatchIndex + 1} of ${findMatches.length}`;

    const m = findMatches[currentMatchIndex];
    const prevFocused = document.activeElement;
    codeEditor.setSelectionRange(m.start, m.end);

    if (prevFocused && (prevFocused === findInput || prevFocused === replaceInput)) {
      prevFocused.focus();
    }

    const textBefore = codeEditor.value.substring(0, m.start);
    const lineIndex = textBefore.split('\n').length - 1;
    const lineHeight = 20;
    const targetScroll = Math.max(0, (lineIndex - 5) * lineHeight);
    codeEditor.scrollTop = targetScroll;
    highlightLayer.scrollTop = targetScroll;
    lineNumbers.scrollTop = targetScroll;
  }

  function nextMatch() {
    if (findMatches.length === 0) return;
    const next = (currentMatchIndex + 1) % findMatches.length;
    highlightMatch(next);
  }

  function prevMatch() {
    if (findMatches.length === 0) return;
    const prev = (currentMatchIndex - 1 + findMatches.length) % findMatches.length;
    highlightMatch(prev);
  }

  function replaceCurrent() {
    if (currentMatchIndex < 0 || currentMatchIndex >= findMatches.length) return;
    const m = findMatches[currentMatchIndex];
    const rep = replaceInput.value;

    codeEditor.value = codeEditor.value.substring(0, m.start) + rep + codeEditor.value.substring(m.end);
    updateHighlight();
    updateLineNumbers();
    editorStatus.textContent = 'Unsaved changes';

    performFind();
  }

  function replaceAll() {
    const query = findInput.value;
    if (!query || findMatches.length === 0) return;
    const rep = replaceInput.value;
    const flags = matchCase ? 'g' : 'gi';
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(escaped, flags);

    codeEditor.value = codeEditor.value.replace(regex, rep);
    updateHighlight();
    updateLineNumbers();
    editorStatus.textContent = 'Unsaved changes';
    performFind();
  }

  // ----------------------------------------------------------------------------
  // 7. File System & Directory Tree Explorer
  // ----------------------------------------------------------------------------
  function getFileIcon(name, isDir) {
    if (isDir) return '📁';
    const ext = name.split('.').pop().toLowerCase();
    switch (ext) {
      case 'tex': return '📄';
      case 'pdf': return '📕';
      case 'png':
      case 'jpg':
      case 'jpeg':
      case 'svg': return '🖼️';
      case 'bib': return '📚';
      case 'py': return '🐍';
      case 'ipynb': return '📓';
      default: return '📝';
    }
  }

  function buildFileTree(list) {
    const root = { name: '', path: '', isDir: true, children: {} };

    list.forEach(f => {
      const parts = f.path.split('/');
      let cur = root;
      let curPath = '';

      parts.forEach((p, idx) => {
        curPath = curPath ? `${curPath}/${p}` : p;
        const isLeaf = (idx === parts.length - 1);

        if (!cur.children[p]) {
          cur.children[p] = {
            name: p,
            path: curPath,
            isDir: isLeaf ? f.is_dir : true,
            children: {},
          };
        }
        cur = cur.children[p];
      });
    });

    return root;
  }

  function renderTree(node, container) {
    const entries = Object.values(node.children);
    entries.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return a.name.localeCompare(b.name);
    });

    entries.forEach(item => {
      const row = document.createElement('div');
      row.className = 'tree-row' + (item.path === currentFile ? ' active' : '');

      if (item.isDir) {
        const isExpanded = folderState[item.path] === true;
        row.innerHTML = `
          <span class="tree-arrow ${isExpanded ? '' : 'collapsed'}">▼</span>
          <span class="tree-icon">${isExpanded ? '📂' : '📁'}</span>
          <span class="tree-label">${escapeHtml(item.name)}</span>
          <span class="tree-action tree-new-file" title="New file in this folder">+</span>
          <span class="tree-delete" title="Delete folder">✕</span>
        `;

        const childContainer = document.createElement('div');
        childContainer.className = 'tree-children' + (isExpanded ? '' : ' hidden');
        renderTree(item, childContainer);

        row.onclick = (e) => {
          if (e.target.classList.contains('tree-delete')) {
            deleteDirectory(item.path);
          } else if (e.target.classList.contains('tree-new-file')) {
            createFileInDir(item.path);
          } else {
            const nextState = !isExpanded;
            folderState[item.path] = nextState;
            renderFileTree();
          }
        };

        container.appendChild(row);
        container.appendChild(childContainer);
      } else {
        row.innerHTML = `
          <span class="tree-arrow empty">•</span>
          <span class="tree-icon">${getFileIcon(item.name, false)}</span>
          <span class="tree-label">${escapeHtml(item.name)}</span>
          <span class="tree-delete" title="Delete">✕</span>
        `;

        row.onclick = (e) => {
          if (e.target.classList.contains('tree-delete')) {
            deleteFile(item.path);
          } else {
            selectFile(item.path);
          }
        };

        container.appendChild(row);
      }
    });
  }

  function renderFileTree() {
    fileTree.innerHTML = '';
    const tree = buildFileTree(files);
    renderTree(tree, fileTree);
  }

  async function loadFiles() {
    try {
      const res = await fetch('/api/files');
      if (res.ok) {
        files = await res.json();
        renderFileTree();

        if (!currentFile || !files.some(f => f.path === currentFile)) {
          const texFiles = files.filter(f => !f.is_dir && f.path.endsWith('.tex'));
          if (texFiles.length > 0) {
            const preferred = texFiles.find(f => f.name === 'main.tex') || texFiles[0];
            await selectFile(preferred.path);
          } else if (files.length > 0) {
            await selectFile(files[0].path);
          } else {
            await selectFile('main.tex');
          }
        }
      }
    } catch (err) {
      console.error('Failed to load files:', err);
    }
  }

  function expandParents(filePath) {
    if (!filePath) return;
    const parts = filePath.split('/');
    let cur = '';
    for (let i = 0; i < parts.length - 1; i++) {
      cur = cur ? `${cur}/${parts[i]}` : parts[i];
      folderState[cur] = true;
    }
  }

  async function selectFile(path) {
    if (path && path.endsWith('.pdf')) {
      pdfFrame.src = `/api/pdf?file=${encodeURIComponent(path)}&t=${Date.now()}`;
      pdfFrame.style.display = 'block';
      pdfEmpty.style.display = 'none';
      return;
    }

    if (currentFile && currentFile !== path && codeEditor.value) {
      await saveFile();
    }
    currentFile = path;
    expandParents(path);
    headerFile.textContent = '• ' + currentFile;
    renderFileTree();
    await loadFileContent(currentFile);
  }

  async function loadFileContent(path) {
    try {
      const res = await fetch('/api/files/content?path=' + encodeURIComponent(path));
      if (res.ok) {
        codeEditor.value = await res.text();
      } else if (res.status === 404 && path === 'main.tex' && files.length === 0) {
        const starter = `\\documentclass{article}
\\usepackage{amsmath}

\\title{LaTeX Document}
\\author{Author}
\\date{\\today}

\\begin{document}
\\maketitle

\\section{Introduction}
Welcome to your local \\LaTeX{} web editor!
\\end{document}
`;
        codeEditor.value = starter;
        await saveFile();
        await loadFiles();
      }
      updateHighlight();
      updateLineNumbers();
      editorStatus.textContent = 'Saved';
    } catch (err) {
      console.error('Failed to load file content:', err);
    }
  }

  async function saveFile() {
    if (!currentFile) return;
    try {
      await fetch('/api/files/content?path=' + encodeURIComponent(currentFile), {
        method: 'POST',
        body: codeEditor.value,
      });
      editorStatus.textContent = 'Saved';
    } catch (err) {
      console.error('Save failed:', err);
    }
  }

  async function deleteFile(path) {
    if (!confirm(`Delete ${path}?`)) return;
    await fetch('/api/files?path=' + encodeURIComponent(path), { method: 'DELETE' });
    await loadFiles();
  }

  async function createFileInDir(dirPath) {
    const input = prompt(`Enter new file in "${dirPath}/" (e.g. chapter1.tex):`);
    if (!input || !input.trim()) return;
    let name = input.trim().replace(/^\/+|\/+$/g, '');
    if (!name.includes('.')) {
      name += '.tex';
    }
    const fullPath = `${dirPath}/${name}`;
    try {
      const res = await fetch('/api/files/content?path=' + encodeURIComponent(fullPath), {
        method: 'POST',
        body: '% ' + name.split('/').pop() + '\n',
      });
      if (res.ok) {
        folderState[dirPath] = true;
        expandParents(fullPath);
        await loadFiles();
        await selectFile(fullPath);
      } else {
        alert('Failed to create file: ' + (await res.text()));
      }
    } catch (err) {
      console.error('Failed to create file in folder:', err);
    }
  }

  async function deleteDirectory(dirPath) {
    if (!confirm(`Delete folder "${dirPath}" and all its contents?`)) return;
    try {
      const res = await fetch('/api/files?path=' + encodeURIComponent(dirPath), { method: 'DELETE' });
      if (res.ok) {
        delete folderState[dirPath];
        await loadFiles();
      } else {
        alert('Failed to delete folder: ' + (await res.text()));
      }
    } catch (err) {
      console.error('Failed to delete directory:', err);
    }
  }

  // ----------------------------------------------------------------------------
  // 8. Compilation & Diagnostics Controller
  // ----------------------------------------------------------------------------
  function getPdfTarget() {
    if (currentFile && currentFile.endsWith('.tex')) {
      return currentFile.replace(/\.tex$/, '.pdf');
    }
    return 'main.pdf';
  }

  async function compileProject() {
    if (isCompiling) return;
    isCompiling = true;
    btnCompile.disabled = true;
    document.getElementById('compileIcon').className = 'spin';
    editorStatus.textContent = 'Compiling...';

    await saveFile();

    try {
      const payload = currentFile ? { main_file: currentFile } : {};
      const res = await fetch('/api/compile', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await res.json();
      handleCompileResult(data);
    } catch (err) {
      console.error('Compilation failed:', err);
      editorStatus.textContent = 'Compile error';
    } finally {
      isCompiling = false;
      btnCompile.disabled = false;
      document.getElementById('compileIcon').className = '';
    }
  }

  function handleCompileResult(result) {
    diagnostics = result.diagnostics || [];
    const errors = diagnostics.filter(d => d.severity === 'error');
    const warnings = diagnostics.filter(d => d.severity === 'warning');

    statErrors.textContent = `${errors.length} Error${errors.length === 1 ? '' : 's'}`;
    statWarnings.textContent = `${warnings.length} Warning${warnings.length === 1 ? '' : 's'}`;
    statDuration.textContent = result.duration_ms ? `(${result.duration_ms}ms)` : '';

    renderDiagnostics(diagnostics);
    updateLineNumbers();

    if (result.success) {
      editorStatus.textContent = 'Compiled successfully';
      const pdfFile = getPdfTarget();
      pdfFrame.src = `/api/pdf?file=${encodeURIComponent(pdfFile)}&t=${Date.now()}`;
      pdfFrame.style.display = 'block';
      pdfEmpty.style.display = 'none';

      if (errors.length === 0) {
        logPanel.classList.add('collapsed');
        logToggle.textContent = '▲';
      }

      loadFiles();
    } else {
      editorStatus.textContent = 'Compilation failed';
      logPanel.classList.remove('collapsed');
      logToggle.textContent = '▼';
    }
  }

  function renderDiagnostics(items) {
    logBody.innerHTML = '';
    if (items.length === 0) {
      const emptyEl = document.createElement('div');
      emptyEl.style.color = 'var(--text-muted)';
      emptyEl.style.fontStyle = 'italic';
      emptyEl.textContent = 'No compilation errors or warnings.';
      logBody.appendChild(emptyEl);
      return;
    }

    items.forEach(d => {
      const fileLabel = d.file ? d.file.split('/').pop() : '';
      const lineLabel = d.line > 0 ? `L${d.line}` : '';
      const loc = [fileLabel, lineLabel].filter(Boolean).join(':');

      const itemEl = document.createElement('div');
      itemEl.className = `diag-item ${d.severity || ''}`;
      itemEl.onclick = () => jumpToDiagnostic(d.file || '', d.line || 0);

      if (loc) {
        const lineSpan = document.createElement('span');
        lineSpan.className = 'diag-line';
        lineSpan.textContent = `[${loc}]`;
        itemEl.appendChild(lineSpan);
        itemEl.appendChild(document.createTextNode(' '));
      }

      const msgSpan = document.createElement('span');
      msgSpan.className = 'diag-msg';
      msgSpan.textContent = d.message || '';
      itemEl.appendChild(msgSpan);

      logBody.appendChild(itemEl);
    });
  }

  window.jumpToDiagnostic = async function(file, line) {
    if (file && file !== currentFile) {
      const match = files.find(f => f.path === file || f.path.endsWith(file));
      if (match) {
        await selectFile(match.path);
      }
    }
    if (line > 0) {
      scrollToEditorLine(line);
    }
  };

  function scrollToEditorLine(lineNumber) {
    const lines = codeEditor.value.split('\n');
    let charOffset = 0;
    for (let i = 0; i < lineNumber - 1 && i < lines.length; i++) {
      charOffset += lines[i].length + 1;
    }

    codeEditor.focus();
    codeEditor.setSelectionRange(charOffset, charOffset);

    const lineHeight = 20;
    const targetScroll = Math.max(0, (lineNumber - 5) * lineHeight);
    codeEditor.scrollTop = targetScroll;
    highlightLayer.scrollTop = targetScroll;
    lineNumbers.scrollTop = targetScroll;
  }

  // ----------------------------------------------------------------------------
  // 9. Bidirectional SyncTeX Navigation
  // ----------------------------------------------------------------------------
  function getCurrentLineNumber() {
    const pos = codeEditor.selectionStart;
    const text = codeEditor.value.substring(0, pos);
    return text.split('\n').length;
  }

  async function syncTexForward() {
    const line = getCurrentLineNumber();
    editorStatus.textContent = `Syncing line ${line}...`;
    try {
      const res = await fetch('/api/synctex/forward', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ file: currentFile, line: line })
      });
      if (res.ok) {
        const fwd = await res.json();
        editorStatus.textContent = `Synced to Page ${fwd.page}`;
        const pdfFile = getPdfTarget();
        pdfFrame.src = `/api/pdf?file=${encodeURIComponent(pdfFile)}&t=${Date.now()}#page=${fwd.page}&view=FitH,${Math.round(fwd.y)}`;
      } else {
        editorStatus.textContent = 'Sync location not found in PDF';
      }
    } catch (err) {
      console.error('SyncTeX failed:', err);
    }
  }

  // ----------------------------------------------------------------------------
  // 10. Event Listeners & Hotkeys Registration
  // ----------------------------------------------------------------------------
  btnNewFile.onclick = async () => {
    const name = prompt('Enter new filename (e.g. chapter1.tex or module4/notes.tex):');
    if (!name || !name.trim()) return;
    let cleanName = name.trim().replace(/^\/+|\/+$/g, '');
    if (!cleanName.includes('.')) {
      cleanName += '.tex';
    }
    try {
      const res = await fetch('/api/files/content?path=' + encodeURIComponent(cleanName), {
        method: 'POST',
        body: '% ' + cleanName.split('/').pop() + '\n',
      });
      if (res.ok) {
        expandParents(cleanName);
        await loadFiles();
        await selectFile(cleanName);
      } else {
        alert('Failed to create file: ' + (await res.text()));
      }
    } catch (err) {
      console.error('Failed to create file:', err);
    }
  };

  if (btnNewFolder) {
    btnNewFolder.onclick = async () => {
      const name = prompt('Enter new subdirectory name (e.g. chapters or module4/notes):');
      if (!name || !name.trim()) return;
      const cleanPath = name.trim().replace(/^\/+|\/+$/g, '');
      try {
        const res = await fetch('/api/directories?path=' + encodeURIComponent(cleanPath), {
          method: 'POST',
        });
        if (res.ok) {
          folderState[cleanPath] = true;
          expandParents(cleanPath);
          await loadFiles();
        } else {
          alert('Failed to create directory: ' + (await res.text()));
        }
      } catch (err) {
        console.error('Failed to create directory:', err);
      }
    };
  }

  btnSave.onclick = saveFile;
  btnCompile.onclick = compileProject;
  btnInitialCompile.onclick = compileProject;

  btnDownload.onclick = () => {
    window.open(`/api/pdf?file=${encodeURIComponent(getPdfTarget())}`, '_blank');
  };
  btnOpenNew.onclick = () => {
    window.open(`/api/pdf?file=${encodeURIComponent(getPdfTarget())}`, '_blank');
  };

  btnSyncTex.onclick = syncTexForward;

  logBar.onclick = () => {
    logPanel.classList.toggle('collapsed');
    logToggle.textContent = logPanel.classList.contains('collapsed') ? '▲' : '▼';
  };

  // Insert Palette Listeners
  btnInsert.onclick = toggleInsertPalette;
  insertClose.onclick = closeInsertPalette;

  insertTabs.addEventListener('click', (e) => {
    const btn = e.target.closest('.insert-tab');
    if (!btn) return;
    insertTabs.querySelectorAll('.insert-tab').forEach(t => t.classList.remove('active'));
    btn.classList.add('active');
    currentCategory = btn.getAttribute('data-cat');
    selectedSnippetIndex = 0;
    renderInsertList();
    insertSearch.focus();
  });

  insertSearch.addEventListener('input', () => {
    selectedSnippetIndex = 0;
    renderInsertList();
  });

  insertSearch.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (filteredSnippets.length > 0) {
        selectedSnippetIndex = (selectedSnippetIndex + 1) % filteredSnippets.length;
        renderInsertList();
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (filteredSnippets.length > 0) {
        selectedSnippetIndex = (selectedSnippetIndex - 1 + filteredSnippets.length) % filteredSnippets.length;
        renderInsertList();
      }
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (filteredSnippets.length > 0 && selectedSnippetIndex >= 0 && filteredSnippets[selectedSnippetIndex]) {
        insertSnippet(filteredSnippets[selectedSnippetIndex]);
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      closeInsertPalette();
    }
  });

  document.addEventListener('mousedown', (e) => {
    if (!insertPalette.classList.contains('hidden') &&
        !insertPalette.contains(e.target) &&
        !btnInsert.contains(e.target)) {
      closeInsertPalette();
    }
  });

  // Find & Replace Listeners
  btnFind.onclick = () => openFind(false);
  findClose.onclick = closeFind;
  findNext.onclick = nextMatch;
  findPrev.onclick = prevMatch;
  btnReplaceOne.onclick = replaceCurrent;
  btnReplaceAll.onclick = replaceAll;

  findToggleReplace.onclick = () => {
    const isHidden = replaceRow.classList.toggle('hidden');
    findToggleReplace.textContent = isHidden ? '▶' : '▼';
    if (!isHidden) {
      replaceInput.focus();
    }
  };

  findCaseSensitive.onclick = () => {
    matchCase = !matchCase;
    findCaseSensitive.classList.toggle('active', matchCase);
    performFind();
  };

  findInput.addEventListener('input', performFind);

  findInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (e.shiftKey) {
        prevMatch();
      } else {
        nextMatch();
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      closeFind();
    }
  });

  replaceInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      replaceCurrent();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      closeFind();
    }
  });

  // Global Keyboard Shortcuts: Alt+I, Ctrl+F, Ctrl+H, Ctrl+S, Ctrl+Enter, Ctrl+J, Escape
  window.addEventListener('keydown', (e) => {
    const isCmdOrCtrl = (e.ctrlKey || e.metaKey);
    if (e.altKey && e.key.toLowerCase() === 'i') {
      e.preventDefault();
      toggleInsertPalette();
    } else if (isCmdOrCtrl && e.shiftKey && e.key.toLowerCase() === 'i') {
      e.preventDefault();
      toggleInsertPalette();
    } else if (isCmdOrCtrl && e.key.toLowerCase() === 'f') {
      e.preventDefault();
      openFind(false);
    } else if (isCmdOrCtrl && e.key.toLowerCase() === 'h') {
      e.preventDefault();
      openFind(true);
    } else if (e.key === 'Escape' && !insertPalette.classList.contains('hidden')) {
      e.preventDefault();
      closeInsertPalette();
    } else if (e.key === 'Escape' && !findWidget.classList.contains('hidden')) {
      e.preventDefault();
      closeFind();
    } else if (isCmdOrCtrl && e.key.toLowerCase() === 's') {
      e.preventDefault();
      saveFile().then(compileProject);
    } else if (isCmdOrCtrl && e.key === 'Enter') {
      e.preventDefault();
      compileProject();
    } else if (isCmdOrCtrl && e.key.toLowerCase() === 'j') {
      e.preventDefault();
      syncTexForward();
    }
  });

  // ----------------------------------------------------------------------------
  // 11. Server-Sent Events (SSE) Live Stream
  // ----------------------------------------------------------------------------
  const evtSource = new EventSource('/api/events');
  evtSource.addEventListener('compile_start', () => {
    isCompiling = true;
    btnCompile.disabled = true;
    document.getElementById('compileIcon').className = 'spin';
  });
  evtSource.addEventListener('compile_done', (e) => {
    try {
      const res = JSON.parse(e.data);
      handleCompileResult(res);
    } catch (err) {}
    isCompiling = false;
    btnCompile.disabled = false;
    document.getElementById('compileIcon').className = '';
  });

  // ----------------------------------------------------------------------------
  // 12. Dynamic Pane Resizers
  // ----------------------------------------------------------------------------
  const mainContainer = document.querySelector('.main-container');
  const sidebar = document.querySelector('aside');
  const editorSection = document.querySelector('.editor-section');
  const viewerSection = document.querySelector('.viewer-section');
  const resizerSidebar = document.getElementById('resizerSidebar');
  const resizerViewer = document.getElementById('resizerViewer');

  // Restore saved layout preferences
  try {
    const savedSidebarWidth = localStorage.getItem('latex_sidebar_width');
    if (savedSidebarWidth && sidebar) {
      const w = parseInt(savedSidebarWidth, 10);
      if (w >= 140 && w <= 700) {
        sidebar.style.width = w + 'px';
      }
    }

    const savedSplitRatio = localStorage.getItem('latex_split_ratio');
    if (savedSplitRatio && editorSection && viewerSection) {
      const ratio = parseFloat(savedSplitRatio);
      if (ratio >= 15 && ratio <= 85) {
        editorSection.style.flex = `${ratio} 1 0px`;
        viewerSection.style.flex = `${100 - ratio} 1 0px`;
      }
    }
  } catch (e) {}

  // Sidebar Resizer Handle
  if (resizerSidebar && sidebar && mainContainer) {
    resizerSidebar.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      const startX = e.clientX;
      const startWidth = sidebar.getBoundingClientRect().width;

      document.body.classList.add('is-resizing');
      resizerSidebar.classList.add('resizing');

      function onPointerMove(moveEvent) {
        const deltaX = moveEvent.clientX - startX;
        let newWidth = startWidth + deltaX;
        const maxWidth = Math.min(600, mainContainer.clientWidth - 420);
        if (newWidth < 140) newWidth = 140;
        if (newWidth > maxWidth) newWidth = maxWidth;

        sidebar.style.width = newWidth + 'px';
        updateLineNumbers();
      }

      function onPointerUp() {
        document.body.classList.remove('is-resizing');
        resizerSidebar.classList.remove('resizing');
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);
        try {
          localStorage.setItem('latex_sidebar_width', sidebar.offsetWidth.toString());
        } catch (err) {}
        updateLineNumbers();
      }

      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
    });

    // Double-click to reset sidebar width to default (220px)
    resizerSidebar.addEventListener('dblclick', () => {
      sidebar.style.width = '220px';
      try {
        localStorage.setItem('latex_sidebar_width', '220');
      } catch (err) {}
      updateLineNumbers();
    });
  }

  // Editor / PDF Viewer Resizer Handle
  if (resizerViewer && editorSection && viewerSection && mainContainer) {
    resizerViewer.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      const startX = e.clientX;
      const editorRect = editorSection.getBoundingClientRect();
      const startEditorWidth = editorRect.width;

      document.body.classList.add('is-resizing');
      resizerViewer.classList.add('resizing');

      function onPointerMove(moveEvent) {
        const deltaX = moveEvent.clientX - startX;
        let newEditorWidth = startEditorWidth + deltaX;
        const resizersWidth = (resizerSidebar ? resizerSidebar.offsetWidth : 5) + resizerViewer.offsetWidth;
        const availWidth = mainContainer.clientWidth - sidebar.offsetWidth - resizersWidth;

        const minEditor = 200;
        const minViewer = 200;
        if (availWidth < (minEditor + minViewer)) return;

        if (newEditorWidth < minEditor) newEditorWidth = minEditor;
        if (newEditorWidth > availWidth - minViewer) newEditorWidth = availWidth - minViewer;

        const ratio = (newEditorWidth / availWidth) * 100;
        editorSection.style.flex = `${ratio} 1 0px`;
        viewerSection.style.flex = `${100 - ratio} 1 0px`;
        updateLineNumbers();
      }

      function onPointerUp() {
        document.body.classList.remove('is-resizing');
        resizerViewer.classList.remove('resizing');
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);

        const curFlex = parseFloat(editorSection.style.flex);
        if (!isNaN(curFlex)) {
          try {
            localStorage.setItem('latex_split_ratio', curFlex.toFixed(2));
          } catch (err) {}
        }
        updateLineNumbers();
      }

      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
    });

    // Double-click to reset editor/viewer split to 50/50
    resizerViewer.addEventListener('dblclick', () => {
      editorSection.style.flex = '50 1 0px';
      viewerSection.style.flex = '50 1 0px';
      try {
        localStorage.setItem('latex_split_ratio', '50');
      } catch (err) {}
      updateLineNumbers();
    });
  }

  // Adjust word-wrapped line heights on window resize
  window.addEventListener('resize', () => {
    updateLineNumbers();
  });

  // ----------------------------------------------------------------------------
  // 13. Application Startup
  // ----------------------------------------------------------------------------
  loadFiles().then(() => {
    if (currentFile) {
      loadFileContent(currentFile).then(compileProject);
    }
  });

})();
