import { debounce, handleError } from './utils';
import type { 
  SearchRequest, 
  SearchResponse, 
  SearchState, 
  APIError,
  SuggestionsResponse,
  FileStats,
  IndexProgress,
  HealthStatus
} from './types';

/**
 * API client for CSV search backend
 */
export class SearchAPI {
  private baseUrl: string;
  private abortController: AbortController | null = null;

  constructor(baseUrl: string = '/api') {
    this.baseUrl = baseUrl;
  }

  /**
   * Cancel any ongoing requests
   */
  cancelRequests(): void {
    if (this.abortController) {
      this.abortController.abort();
    }
  }

  /**
   * Make HTTP request with error handling
   */
  private async request<T>(
    endpoint: string, 
    options: RequestInit = {}
  ): Promise<T> {
    // Cancel previous request
    this.cancelRequests();
    
    // Create new abort controller
    this.abortController = new AbortController();
    
    const url = `${this.baseUrl}${endpoint}`;
    const requestOptions: RequestInit = {
      ...options,
      signal: this.abortController.signal,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    };

    try {
      const response = await fetch(url, requestOptions);
      
      if (!response.ok) {
        const errorData: APIError = await response.json().catch(() => ({
          error: 'Request failed',
          code: response.status,
          message: response.statusText,
        }));
        throw new Error(errorData.message || errorData.error || 'Request failed');
      }

      return await response.json();
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        throw new Error('Request cancelled');
      }
      throw error;
    }
  }

  /**
   * Search CSV data
   */
  async search(params: SearchRequest): Promise<SearchResponse> {
    const searchParams = new URLSearchParams();
    
    if (params.query) searchParams.set('q', params.query);
    if (params.page) searchParams.set('page', params.page.toString());
    if (params.limit) searchParams.set('limit', params.limit.toString());
    if (params.columns && params.columns.length > 0) {
      searchParams.set('columns', params.columns.join(','));
    }

    return this.request<SearchResponse>(`/search?${searchParams.toString()}`);
  }

  /**
   * Get available columns
   */
  async getColumns(): Promise<string[]> {
    const response = await this.request<{ columns: string[] }>('/columns');
    return response.columns;
  }

  /**
   * Get file statistics
   */
  async getStats(): Promise<FileStats> {
    return this.request<FileStats>('/stats');
  }

  /**
   * Get indexing progress
   */
  async getProgress(): Promise<IndexProgress> {
    return this.request<IndexProgress>('/progress');
  }

  /**
   * Get search suggestions
   */
  async getSuggestions(query: string, limit: number = 10): Promise<string[]> {
    const params = new URLSearchParams();
    params.set('q', query);
    params.set('limit', limit.toString());
    
    const response = await this.request<SuggestionsResponse>(`/suggestions?${params.toString()}`);
    return response.suggestions;
  }

  /**
   * Get health status
   */
  async getHealth(): Promise<HealthStatus> {
    return this.request<HealthStatus>('/health');
  }
}

/**
 * Search manager class
 */
export class SearchManager {
  private api: SearchAPI;
  private state: SearchState;
  private stateListeners: Set<(state: SearchState) => void> = new Set();
  private debouncedSearch: (query: string) => void;

  constructor(api: SearchAPI) {
    this.api = api;
    this.state = {
      query: '',
      results: [],
      total: 0,
      page: 1,
      limit: 50,
      totalPages: 0,
      loading: false,
      error: null,
      duration: 0,
    };

    // Create debounced search function
    this.debouncedSearch = debounce((query: string) => {
      this.performSearch(query);
    }, 300);
  }

  /**
   * Get current search state
   */
  getState(): SearchState {
    return { ...this.state };
  }

  /**
   * Subscribe to state changes
   */
  subscribe(listener: (state: SearchState) => void): () => void {
    this.stateListeners.add(listener);
    return () => this.stateListeners.delete(listener);
  }

  /**
   * Update state and notify listeners
   */
  private updateState(updates: Partial<SearchState>): void {
    this.state = { ...this.state, ...updates };
    this.stateListeners.forEach(listener => listener(this.state));
  }

  /**
   * Set search query
   */
  setQuery(query: string): void {
    this.updateState({ query, page: 1, error: null });
    
    if (query.trim()) {
      this.debouncedSearch(query);
    } else {
      this.updateState({ 
        results: [], 
        total: 0, 
        totalPages: 0, 
        loading: false 
      });
    }
  }

  /**
   * Set page
   */
  setPage(page: number): void {
    if (page < 1 || page > this.state.totalPages) return;
    
    this.updateState({ page });
    this.performSearch(this.state.query, page);
  }

  /**
   * Set page size
   */
  setPageSize(limit: number): void {
    this.updateState({ limit, page: 1 });
    if (this.state.query.trim()) {
      this.performSearch(this.state.query, 1, limit);
    }
  }

  /**
   * Perform search
   */
  private async performSearch(query: string, page: number = 1, limit?: number): Promise<void> {
    if (!query.trim()) {
      this.updateState({ 
        results: [], 
        total: 0, 
        totalPages: 0, 
        loading: false 
      });
      return;
    }

    this.updateState({ loading: true, error: null });

    try {
      const response = await this.api.search({
        query,
        page,
        limit: limit || this.state.limit,
      });

      this.updateState({
        results: response.results,
        total: response.total,
        page: response.page,
        limit: response.limit,
        totalPages: response.total_pages,
        loading: false,
        duration: response.duration_ms,
        error: null,
      });
    } catch (error) {
      this.updateState({
        loading: false,
        error: handleError(error, 'search'),
      });
    }
  }

  /**
   * Clear search
   */
  clearSearch(): void {
    this.api.cancelRequests();
    this.updateState({
      query: '',
      results: [],
      total: 0,
      page: 1,
      totalPages: 0,
      loading: false,
      error: null,
      duration: 0,
    });
  }

  /**
   * Refresh current search
   */
  refresh(): void {
    if (this.state.query.trim()) {
      this.performSearch(this.state.query, this.state.page);
    }
  }
}

/**
 * Application state manager
 */
export class AppStateManager {
  private searchManager: SearchManager;
  private api: SearchAPI;
  private state: {
    columns: string[];
    stats: FileStats | null;
    progress: IndexProgress | null;
    health: HealthStatus | null;
    suggestions: string[];
    darkMode: boolean;
  };
  private stateListeners: Set<(state: any) => void> = new Set();

  constructor(api: SearchAPI) {
    this.api = api;
    this.searchManager = new SearchManager(api);
    this.state = {
      columns: [],
      stats: null,
      progress: null,
      health: null,
      suggestions: [],
      darkMode: this.loadDarkMode(),
    };
  }

  /**
   * Initialize application state
   */
  async initialize(): Promise<void> {
    try {
      // Load initial data in parallel
      const [columns, stats, health] = await Promise.allSettled([
        this.api.getColumns(),
        this.api.getStats(),
        this.api.getHealth(),
      ]);

      if (columns.status === 'fulfilled') {
        this.state.columns = columns.value;
      }

      if (stats.status === 'fulfilled') {
        this.state.stats = stats.value;
      }

      if (health.status === 'fulfilled') {
        this.state.health = health.value;
      }

      this.notifyListeners();
    } catch (error) {
      console.error('Failed to initialize app state:', error);
    }
  }

  /**
   * Get search manager
   */
  getSearchManager(): SearchManager {
    return this.searchManager;
  }

  /**
   * Get current state
   */
  getState() {
    return {
      ...this.state,
      search: this.searchManager.getState(),
    };
  }

  /**
   * Subscribe to state changes
   */
  subscribe(listener: (state: any) => void): () => void {
    this.stateListeners.add(listener);
    return () => this.stateListeners.delete(listener);
  }

  /**
   * Notify state listeners
   */
  private notifyListeners(): void {
    this.stateListeners.forEach(listener => listener(this.getState()));
  }

  /**
   * Get search suggestions
   */
  async getSuggestions(query: string): Promise<void> {
    if (!query.trim()) {
      this.state.suggestions = [];
      this.notifyListeners();
      return;
    }

    try {
      const suggestions = await this.api.getSuggestions(query);
      this.state.suggestions = suggestions;
      this.notifyListeners();
    } catch (error) {
      console.error('Failed to get suggestions:', error);
    }
  }

  /**
   * Toggle dark mode
   */
  toggleDarkMode(): void {
    this.state.darkMode = !this.state.darkMode;
    this.saveDarkMode(this.state.darkMode);
    this.applyDarkMode(this.state.darkMode);
    this.notifyListeners();
  }

  /**
   * Load dark mode preference
   */
  private loadDarkMode(): boolean {
    const saved = localStorage.getItem('darkMode');
    if (saved !== null) {
      return JSON.parse(saved);
    }
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  }

  /**
   * Save dark mode preference
   */
  private saveDarkMode(darkMode: boolean): void {
    localStorage.setItem('darkMode', JSON.stringify(darkMode));
  }

  /**
   * Apply dark mode to document
   */
  private applyDarkMode(darkMode: boolean): void {
    if (darkMode) {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }

  /**
   * Get progress updates
   */
  async getProgress(): Promise<void> {
    try {
      const progress = await this.api.getProgress();
      this.state.progress = progress;
      this.notifyListeners();
    } catch (error) {
      console.error('Failed to get progress:', error);
    }
  }
}
