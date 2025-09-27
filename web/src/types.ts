// API Response Types
export interface SearchRequest {
  query: string;
  page?: number;
  limit?: number;
  columns?: string[];
}

export interface SearchResponse {
  results: SearchResult[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
  query: string;
  duration_ms: number;
}

export interface SearchResult {
  row: number;
  data: Record<string, string>;
  score?: number;
  highlights?: Record<string, string[]>;
}

export interface ColumnInfo {
  name: string;
  index: number;
  type?: string;
}

export interface FileStats {
  row_count: number;
  column_count: number;
  file_size_bytes: number;
  indexed_at: string;
  columns: ColumnInfo[];
}

export interface IndexProgress {
  processed_rows: number;
  total_rows: number;
  progress_percent: number;
  is_complete: boolean;
  error?: string;
}

export interface APIError {
  error: string;
  code: number;
  message?: string;
}

export interface HealthStatus {
  status: string;
  timestamp: string;
  indexed: boolean;
}

export interface SuggestionsResponse {
  suggestions: string[];
}

// UI State Types
export interface SearchState {
  query: string;
  results: SearchResult[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
  loading: boolean;
  error: string | null;
  duration: number;
}

export interface AppState {
  search: SearchState;
  columns: string[];
  stats: FileStats | null;
  progress: IndexProgress | null;
  health: HealthStatus | null;
  suggestions: string[];
  darkMode: boolean;
}

// Component Props Types
export interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  onSearch: () => void;
  loading: boolean;
  suggestions: string[];
  onSuggestionSelect: (suggestion: string) => void;
}

export interface SearchResultsProps {
  results: SearchResult[];
  loading: boolean;
  error: string | null;
  total: number;
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  columns: string[];
}

export interface SearchResultRowProps {
  result: SearchResult;
  columns: string[];
  query: string;
}

export interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  disabled?: boolean;
}

export interface StatsDisplayProps {
  stats: FileStats | null;
  progress: IndexProgress | null;
  health: HealthStatus | null;
}

// Utility Types
export type SortDirection = 'asc' | 'desc';

export interface SortConfig {
  column: string;
  direction: SortDirection;
}

export interface FilterConfig {
  column: string;
  value: string;
  operator: 'contains' | 'equals' | 'startsWith' | 'endsWith';
}

// Event Types
export interface SearchEvent {
  type: 'search';
  query: string;
  timestamp: number;
  duration: number;
  resultCount: number;
}

export interface ErrorEvent {
  type: 'error';
  error: string;
  timestamp: number;
  context?: string;
}
