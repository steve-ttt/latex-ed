export type Severity = 'error' | 'warning' | 'info';

export interface Diagnostic {
  severity: Severity;
  file: string;
  line: number;
  message: string;
  context?: string;
}

export interface CompileResult {
  success: boolean;
  pdf_file?: string;
  synctex_file?: string;
  diagnostics: Diagnostic[];
  raw_log?: string;
  duration_ms: number;
}

export interface FileInfo {
  path: string;
  name: string;
  is_dir: boolean;
  size: number;
  mod_time: number;
}
