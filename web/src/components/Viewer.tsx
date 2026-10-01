import React from 'react';
import { RefreshCw, Download, ExternalLink, FileText } from 'lucide-react';

interface ViewerProps {
  pdfUrl: string;
  pdfAvailable: boolean;
  onRefresh: () => void;
  isCompiling: boolean;
}

export const Viewer: React.FC<ViewerProps> = ({
  pdfUrl,
  pdfAvailable,
  onRefresh,
  isCompiling,
}) => {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: '#252526' }}>
      {/* Top Toolbar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '6px 12px',
          background: '#2d2d2d',
          borderBottom: '1px solid #3c3c3c',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px', color: '#cccccc' }}>
          <FileText size={16} color="#007acc" />
          <span>PDF Preview</span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <button
            onClick={onRefresh}
            disabled={isCompiling}
            title="Recompile (Ctrl+Enter)"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '4px 10px',
              fontSize: '12px',
              borderRadius: '4px',
              border: 'none',
              background: '#0e639c',
              color: 'white',
              cursor: isCompiling ? 'not-allowed' : 'pointer',
              opacity: isCompiling ? 0.6 : 1,
            }}
          >
            <RefreshCw size={13} className={isCompiling ? 'spin' : ''} />
            {isCompiling ? 'Compiling...' : 'Recompile'}
          </button>

          {pdfAvailable && (
            <>
              <a
                href={pdfUrl}
                download="document.pdf"
                title="Download PDF"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  padding: '4px 8px',
                  borderRadius: '4px',
                  background: '#3c3c3c',
                  color: '#cccccc',
                  textDecoration: 'none',
                }}
              >
                <Download size={14} />
              </a>

              <a
                href={pdfUrl}
                target="_blank"
                rel="noreferrer"
                title="Open in new window"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  padding: '4px 8px',
                  borderRadius: '4px',
                  background: '#3c3c3c',
                  color: '#cccccc',
                  textDecoration: 'none',
                }}
              >
                <ExternalLink size={14} />
              </a>
            </>
          )}
        </div>
      </div>

      {/* PDF View Container */}
      <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
        {pdfAvailable ? (
          <iframe
            key={pdfUrl}
            src={`${pdfUrl}#toolbar=1&navpanes=0`}
            style={{
              width: '100%',
              height: '100%',
              border: 'none',
              background: '#525659',
            }}
            title="PDF Document"
          />
        ) : (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
              color: '#888888',
              gap: '12px',
            }}
          >
            <FileText size={48} strokeWidth={1.5} />
            <p style={{ fontSize: '14px' }}>No compiled PDF available.</p>
            <button
              onClick={onRefresh}
              style={{
                padding: '6px 14px',
                borderRadius: '4px',
                border: 'none',
                background: '#0e639c',
                color: 'white',
                cursor: 'pointer',
              }}
            >
              Compile Now
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
