import React, { useState } from 'react';
import { File, Folder, Plus, Trash2, FileCode } from 'lucide-react';
import { FileInfo } from '../types';

interface SidebarProps {
  files: FileInfo[];
  currentFile: string;
  onSelectFile: (path: string) => void;
  onCreateFile: (name: string) => void;
  onDeleteFile: (path: string) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  files,
  currentFile,
  onSelectFile,
  onCreateFile,
  onDeleteFile,
}) => {
  const [newFileName, setNewFileName] = useState('');
  const [showInput, setShowInput] = useState(false);

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (newFileName.trim()) {
      onCreateFile(newFileName.trim());
      setNewFileName('');
      setShowInput(false);
    }
  };

  return (
    <div
      style={{
        width: '220px',
        height: '100%',
        background: '#252526',
        borderRight: '1px solid #3c3c3c',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* Header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 14px',
          borderBottom: '1px solid #333333',
        }}
      >
        <span style={{ fontSize: '11px', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.5px', color: '#bbbbbb' }}>
          Project Files
        </span>
        <button
          onClick={() => setShowInput(!showInput)}
          title="New File"
          style={{
            background: 'none',
            border: 'none',
            color: '#cccccc',
            cursor: 'pointer',
            padding: '2px',
          }}
        >
          <Plus size={16} />
        </button>
      </div>

      {showInput && (
        <form onSubmit={handleCreate} style={{ padding: '8px 12px', borderBottom: '1px solid #333333' }}>
          <input
            autoFocus
            type="text"
            placeholder="filename.tex"
            value={newFileName}
            onChange={(e) => setNewFileName(e.target.value)}
            onBlur={() => !newFileName && setShowInput(false)}
            style={{
              width: '100%',
              padding: '4px 6px',
              fontSize: '12px',
              background: '#3c3c3c',
              color: 'white',
              border: '1px solid #007acc',
              borderRadius: '3px',
              outline: 'none',
            }}
          />
        </form>
      )}

      {/* File List */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '6px 0' }}>
        {files.map((file) => {
          const isSelected = file.path === currentFile;
          const isTeX = file.path.endsWith('.tex');
          return (
            <div
              key={file.path}
              onClick={() => onSelectFile(file.path)}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '6px 14px',
                fontSize: '13px',
                cursor: 'pointer',
                background: isSelected ? '#37373d' : 'transparent',
                color: isSelected ? '#ffffff' : '#cccccc',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', overflow: 'hidden' }}>
                {file.is_dir ? (
                  <Folder size={15} color="#dcb67a" />
                ) : isTeX ? (
                  <FileCode size={15} color="#4ec9b0" />
                ) : (
                  <File size={15} color="#9cdcfe" />
                )}
                <span style={{ textOverflow: 'ellipsis', overflow: 'hidden', whiteSpace: 'nowrap' }}>
                  {file.path}
                </span>
              </div>

              {file.path !== 'main.tex' && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    if (confirm(`Delete ${file.path}?`)) {
                      onDeleteFile(file.path);
                    }
                  }}
                  title="Delete file"
                  style={{
                    background: 'none',
                    border: 'none',
                    color: '#888888',
                    cursor: 'pointer',
                    padding: '2px',
                    opacity: isSelected ? 1 : 0.4,
                  }}
                >
                  <Trash2 size={13} />
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
