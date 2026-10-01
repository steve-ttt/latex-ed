import React, { useState } from 'react';
import { AlertCircle, AlertTriangle, CheckCircle, ChevronDown, ChevronUp, Terminal } from 'lucide-react';
import { Diagnostic, CompileResult } from '../types';

interface LogPanelProps {
  result: CompileResult | null;
  isOpen: boolean;
  onToggle: () => void;
}

export const LogPanel: React.FC<LogPanelProps> = ({ result, isOpen, onToggle }) => {
  const [tab, setTab] = useState<'diagnostics' | 'raw'>('diagnostics');

  const errors = result?.diagnostics.filter((d) => d.severity === 'error') || [];
  const warnings = result?.diagnostics.filter((d) => d.severity === 'warning') || [];

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        background: '#1e1e1e',
        borderTop: '1px solid #333333',
        height: isOpen ? '220px' : '30px',
        transition: 'height 0.15s ease',
        overflow: 'hidden',
      }}
    >
      {/* Bar Header */}
      <div
        onClick={onToggle}
        style={{
          height: '30px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 12px',
          background: '#252526',
          cursor: 'pointer',
          userSelect: 'none',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px', fontSize: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            {result?.success ? (
              <CheckCircle size={14} color="#4ec9b0" />
            ) : errors.length > 0 ? (
              <AlertCircle size={14} color="#f14c4c" />
            ) : (
              <Terminal size={14} color="#888888" />
            )}
            <span style={{ fontWeight: 600, color: '#cccccc' }}>Compilation Logs & Diagnostics</span>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            <span style={{ color: errors.length > 0 ? '#f14c4c' : '#888888' }}>
              {errors.length} {errors.length === 1 ? 'Error' : 'Errors'}
            </span>
            <span style={{ color: warnings.length > 0 ? '#cca700' : '#888888' }}>
              {warnings.length} {warnings.length === 1 ? 'Warning' : 'Warnings'}
            </span>
            {result && (
              <span style={{ color: '#888888' }}>({result.duration_ms}ms)</span>
            )}
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          {isOpen ? <ChevronDown size={16} /> : <ChevronUp size={16} />}
        </div>
      </div>

      {/* Tabs & Content */}
      {isOpen && (
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          <div style={{ display: 'flex', background: '#2d2d2d', borderBottom: '1px solid #333333' }}>
            <button
              onClick={() => setTab('diagnostics')}
              style={{
                padding: '4px 12px',
                fontSize: '12px',
                background: tab === 'diagnostics' ? '#1e1e1e' : 'transparent',
                color: tab === 'diagnostics' ? '#ffffff' : '#888888',
                border: 'none',
                cursor: 'pointer',
              }}
            >
              Issues ({errors.length + warnings.length})
            </button>
            <button
              onClick={() => setTab('raw')}
              style={{
                padding: '4px 12px',
                fontSize: '12px',
                background: tab === 'raw' ? '#1e1e1e' : 'transparent',
                color: tab === 'raw' ? '#ffffff' : '#888888',
                border: 'none',
                cursor: 'pointer',
              }}
            >
              Raw TeX Output
            </button>
          </div>

          <div style={{ flex: 1, overflowY: 'auto', padding: '8px 12px', fontSize: '12px' }}>
            {tab === 'diagnostics' ? (
              result?.diagnostics.length === 0 ? (
                <div style={{ color: '#888888', fontStyle: 'italic', padding: '8px 0' }}>
                  No errors or warnings reported. Clean build!
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  {result?.diagnostics.map((d: Diagnostic, i: number) => (
                    <div
                      key={i}
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: '8px',
                        padding: '6px 8px',
                        borderRadius: '3px',
                        background: d.severity === 'error' ? 'rgba(241, 76, 76, 0.1)' : 'rgba(204, 167, 0, 0.1)',
                        borderLeft: `3px solid ${d.severity === 'error' ? '#f14c4c' : '#cca700'}`,
                      }}
                    >
                      {d.severity === 'error' ? (
                        <AlertCircle size={14} color="#f14c4c" style={{ marginTop: '2px', flexShrink: 0 }} />
                      ) : (
                        <AlertTriangle size={14} color="#cca700" style={{ marginTop: '2px', flexShrink: 0 }} />
                      )}
                      <div>
                        <div style={{ fontWeight: 500, color: d.severity === 'error' ? '#f48771' : '#dcdcaa' }}>
                          {d.file ? `${d.file}:${d.line}` : d.line ? `Line ${d.line}` : 'General'}: {d.message}
                        </div>
                        {d.context && (
                          <div style={{ color: '#888888', fontFamily: 'monospace', marginTop: '2px' }}>
                            {d.context}
                          </div>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )
            ) : (
              <pre
                style={{
                  fontFamily: 'monospace',
                  fontSize: '11px',
                  color: '#cccccc',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {result?.raw_log || 'No compilation output yet.'}
              </pre>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
