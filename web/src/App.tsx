import React, { useState, useEffect, useCallback } from 'react';
import { Sidebar } from './components/Sidebar';
import { Editor } from './components/Editor';
import { Viewer } from './components/Viewer';
import { LogPanel } from './components/LogPanel';
import { FileInfo, Diagnostic, CompileResult } from './types';
import { Play, Save } from 'lucide-react';

export const App: React.FC = () => {
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [currentFile, setCurrentFile] = useState<string>('main.tex');
  const [content, setContent] = useState<string>('');
  const [isCompiling, setIsCompiling] = useState<boolean>(false);
  const [compileResult, setCompileResult] = useState<CompileResult | null>(null);
  const [pdfTimestamp, setPdfTimestamp] = useState<number>(Date.now());
  const [pdfAvailable, setPdfAvailable] = useState<boolean>(false);
  const [isLogOpen, setIsLogOpen] = useState<boolean>(false);

  // 1. Fetch file list
  const loadFiles = useCallback(async () => {
    try {
      const res = await fetch('/api/files');
      if (res.ok) {
        const data: FileInfo[] = await res.json();
        setFiles(data);

        // If main.tex exists, select it, otherwise select the first file or initialize
        if (data.length > 0) {
          const hasMain = data.some((f) => f.path === 'main.tex');
          if (!hasMain && data[0]) {
            setCurrentFile(data[0].path);
          }
        }
      }
    } catch (e) {
      console.error('Failed to load files', e);
    }
  }, []);

  // 2. Load file content
  const loadContent = useCallback(async (filePath: string) => {
    try {
      const res = await fetch(`/api/files/content?path=${encodeURIComponent(filePath)}`);
      if (res.ok) {
        const text = await res.text();
        setContent(text);
      } else if (res.status === 404 && filePath === 'main.tex') {
        // Initialize default template if empty
        const defaultTex = `\\documentclass{article}
\\usepackage{amsmath}
\\usepackage{graphicx}

\\title{My LaTeX Document}
\\author{Stephen}
\\date{\\today}

\\begin{document}

\\maketitle

\\section{Introduction}
Welcome to your local \\LaTeX{} web editor!
Edit your document on the left, and see real-time PDF output on the right.

\\begin{equation}
  E = mc^2
\\end{equation}

\\end{document}
`;
        await saveContent(filePath, defaultTex);
        setContent(defaultTex);
        loadFiles();
      }
    } catch (e) {
      console.error('Failed to read file content', e);
    }
  }, [loadFiles]);

  // 3. Save file content
  const saveContent = async (filePath: string, textToSave: string) => {
    try {
      await fetch(`/api/files/content?path=${encodeURIComponent(filePath)}`, {
        method: 'POST',
        body: textToSave,
      });
    } catch (e) {
      console.error('Failed to save file', e);
    }
  };

  // 4. Trigger Compile
  const handleCompile = async () => {
    setIsCompiling(true);
    // Save current file first
    await saveContent(currentFile, content);

    try {
      const res = await fetch('/api/compile', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ main_file: 'main.tex' }),
      });
      if (res.ok) {
        const result: CompileResult = await res.json();
        setCompileResult(result);
        if (result.success) {
          setPdfAvailable(true);
          setPdfTimestamp(Date.now());
        } else {
          setIsLogOpen(true);
        }
      }
    } catch (e) {
      console.error('Compilation failed', e);
    } finally {
      setIsCompiling(false);
    }
  };

  // Initial load
  useEffect(() => {
    loadFiles();
  }, [loadFiles]);

  useEffect(() => {
    if (currentFile) {
      loadContent(currentFile);
    }
  }, [currentFile, loadContent]);

  // Listen to SSE events from server
  useEffect(() => {
    const eventSource = new EventSource('/api/events');

    eventSource.addEventListener('compile_start', () => {
      setIsCompiling(true);
    });

    eventSource.addEventListener('compile_done', (e) => {
      setIsCompiling(false);
      try {
        const result: CompileResult = JSON.parse(e.data);
        setCompileResult(result);
        if (result.success) {
          setPdfAvailable(true);
          setPdfTimestamp(Date.now());
        }
      } catch (err) {
        console.error('Error parsing compile_done event', err);
      }
    });

    return () => {
      eventSource.close();
    };
  }, []);

  // Global hotkeys (Ctrl+S to save & compile, Ctrl+Enter to compile)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        saveContent(currentFile, content);
        handleCompile();
      } else if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        handleCompile();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [currentFile, content]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', width: '100vw' }}>
      {/* Top Navbar */}
      <div
        style={{
          height: '42px',
          background: '#1e1e1e',
          borderBottom: '1px solid #333333',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 16px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <span style={{ fontWeight: 700, fontSize: '15px', color: '#4ec9b0', letterSpacing: '0.5px' }}>
            LaTeX-Ed
          </span>
          <span style={{ fontSize: '12px', color: '#888888' }}>
            • {currentFile}
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <button
            onClick={() => saveContent(currentFile, content)}
            title="Save file (Ctrl+S)"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '5px 12px',
              fontSize: '12px',
              background: '#3c3c3c',
              color: '#cccccc',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
            }}
          >
            <Save size={13} />
            Save
          </button>

          <button
            onClick={handleCompile}
            disabled={isCompiling}
            title="Compile Project (Ctrl+Enter)"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '5px 14px',
              fontSize: '12px',
              fontWeight: 600,
              background: isCompiling ? '#004c7a' : '#0e639c',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: isCompiling ? 'not-allowed' : 'pointer',
            }}
          >
            <Play size={13} fill="white" />
            {isCompiling ? 'Compiling...' : 'Recompile'}
          </button>
        </div>
      </div>

      {/* Main Workspace */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        <Sidebar
          files={files}
          currentFile={currentFile}
          onSelectFile={(f) => {
            saveContent(currentFile, content);
            setCurrentFile(f);
          }}
          onCreateFile={async (name) => {
            let initial = '';
            if (name.endsWith('.tex') || !name.includes('.')) {
              try {
                const tplRes = await fetch(`/api/template?path=${encodeURIComponent(name)}`);
                if (tplRes.ok) {
                  initial = await tplRes.text();
                }
              } catch (_) {}
            }
            await saveContent(name, initial);
            await loadFiles();
            setCurrentFile(name);
          }}
          onDeleteFile={async (path) => {
            await fetch(`/api/files?path=${encodeURIComponent(path)}`, { method: 'DELETE' });
            await loadFiles();
            if (currentFile === path) {
              setCurrentFile('main.tex');
            }
          }}
        />

        {/* Center: Editor + Bottom Log Panel */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%', borderRight: '1px solid #333333' }}>
          <div style={{ flex: 1, height: 'calc(100% - 30px)', overflow: 'hidden' }}>
            <Editor
              content={content}
              onChange={setContent}
              onSave={() => {
                saveContent(currentFile, content);
                handleCompile();
              }}
              diagnostics={compileResult?.diagnostics || []}
              currentFile={currentFile}
            />
          </div>

          <LogPanel
            result={compileResult}
            isOpen={isLogOpen}
            onToggle={() => setIsLogOpen(!isLogOpen)}
          />
        </div>

        {/* Right: PDF Viewer */}
        <div style={{ flex: 1, height: '100%' }}>
          <Viewer
            pdfUrl={`/api/pdf?t=${pdfTimestamp}`}
            pdfAvailable={pdfAvailable}
            onRefresh={handleCompile}
            isCompiling={isCompiling}
          />
        </div>
      </div>
    </div>
  );
};
