import { SearchAPI, SearchManager, AppStateManager } from './search';
import { formatNumber, formatDuration, formatFileSize, highlightText, downloadCSV, handleError } from './utils';
import type { SearchResult, AppState } from './types';

/**
 * Main application class
 */
class CSVSearchApp {
  private api: SearchAPI;
  private appState: AppStateManager;
  private searchManager: SearchManager;
  private currentState: AppState | null = null;

  // DOM elements
  private elements: {
    loadingScreen: HTMLElement;
    app: HTMLElement;
    searchInput: HTMLInputElement;
    searchLoading: HTMLElement;
    clearSearch: HTMLElement;
    suggestions: HTMLElement;
    suggestionsList: HTMLElement;
    searchStats: HTMLElement;
    resultCount: HTMLElement;
    searchDuration: HTMLElement;
    resultsHeader: HTMLElement;
    pageSize: HTMLSelectElement;
    paginationInfo: HTMLElement;
    errorMessage: HTMLElement;
    errorText: HTMLElement;
    emptyState: HTMLElement;
    resultsTable: HTMLElement;
    tableHeader: HTMLElement;
    tableBody: HTMLElement;
    loadingState: HTMLElement;
    pagination: HTMLElement;
    prevPage: HTMLElement;
    nextPage: HTMLElement;
    pageNumbers: HTMLElement;
    currentPage: HTMLElement;
    totalPages: HTMLElement;
    fileStats: HTMLElement;
    darkModeToggle: HTMLElement;
    exportBtn: HTMLElement;
    progressBar: HTMLElement;
    progressFill: HTMLElement;
  };

  constructor() {
    this.api = new SearchAPI();
    this.appState = new AppStateManager(this.api);
    this.searchManager = this.appState.getSearchManager();
    
    this.elements = this.initializeElements();
    this.setupEventListeners();
    this.initializeApp();
  }

  /**
   * Initialize DOM elements
   */
  private initializeElements() {
    return {
      loadingScreen: document.getElementById('loading-screen')!,
      app: document.getElementById('app')!,
      searchInput: document.getElementById('search-input') as HTMLInputElement,
      searchLoading: document.getElementById('search-loading')!,
      clearSearch: document.getElementById('clear-search')!,
      suggestions: document.getElementById('suggestions')!,
      suggestionsList: document.getElementById('suggestions-list')!,
      searchStats: document.getElementById('search-stats')!,
      resultCount: document.getElementById('result-count')!,
      searchDuration: document.getElementById('search-duration')!,
      resultsHeader: document.getElementById('results-header')!,
      pageSize: document.getElementById('page-size') as HTMLSelectElement,
      paginationInfo: document.getElementById('pagination-info')!,
      errorMessage: document.getElementById('error-message')!,
      errorText: document.getElementById('error-text')!,
      emptyState: document.getElementById('empty-state')!,
      resultsTable: document.getElementById('results-table')!,
      tableHeader: document.getElementById('table-header')!,
      tableBody: document.getElementById('table-body')!,
      loadingState: document.getElementById('loading-state')!,
      pagination: document.getElementById('pagination')!,
      prevPage: document.getElementById('prev-page')!,
      nextPage: document.getElementById('next-page')!,
      pageNumbers: document.getElementById('page-numbers')!,
      currentPage: document.getElementById('current-page')!,
      totalPages: document.getElementById('total-pages')!,
      fileStats: document.getElementById('file-stats')!,
      darkModeToggle: document.getElementById('dark-mode-toggle')!,
      exportBtn: document.getElementById('export-btn')!,
      progressBar: document.getElementById('progress-bar')!,
      progressFill: document.getElementById('progress-fill')!,
    };
  }

  /**
   * Setup event listeners
   */
  private setupEventListeners(): void {
    // Search input
    this.elements.searchInput.addEventListener('input', (e) => {
      const target = e.target as HTMLInputElement;
      this.searchManager.setQuery(target.value);
      this.updateSuggestions(target.value);
    });

    this.elements.searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        this.hideSuggestions();
      }
    });

    // Clear search
    this.elements.clearSearch.addEventListener('click', () => {
      this.elements.searchInput.value = '';
      this.searchManager.clearSearch();
      this.hideSuggestions();
      this.elements.searchInput.focus();
    });

    // Page size
    this.elements.pageSize.addEventListener('change', (e) => {
      const target = e.target as HTMLSelectElement;
      this.searchManager.setPageSize(parseInt(target.value));
    });

    // Pagination
    this.elements.prevPage.addEventListener('click', () => {
      if (this.currentState && this.currentState.search.page > 1) {
        this.searchManager.setPage(this.currentState.search.page - 1);
      }
    });

    this.elements.nextPage.addEventListener('click', () => {
      if (this.currentState && this.currentState.search.page < this.currentState.search.totalPages) {
        this.searchManager.setPage(this.currentState.search.page + 1);
      }
    });

    // Dark mode toggle
    this.elements.darkModeToggle.addEventListener('click', () => {
      this.appState.toggleDarkMode();
    });

    // Export button
    this.elements.exportBtn.addEventListener('click', () => {
      this.exportResults();
    });

    // Hide suggestions when clicking outside
    document.addEventListener('click', (e) => {
      if (!this.elements.suggestions.contains(e.target as Node) && 
          !this.elements.searchInput.contains(e.target as Node)) {
        this.hideSuggestions();
      }
    });

    // Keyboard shortcuts
    document.addEventListener('keydown', (e) => {
      if (e.ctrlKey || e.metaKey) {
        switch (e.key) {
          case 'k':
            e.preventDefault();
            this.elements.searchInput.focus();
            break;
          case 'e':
            e.preventDefault();
            this.exportResults();
            break;
        }
      }
    });
  }

  /**
   * Initialize application
   */
  private async initializeApp(): Promise<void> {
    try {
      // Subscribe to state changes
      this.appState.subscribe((state) => {
        this.currentState = state;
        this.updateUI(state);
      });

      this.searchManager.subscribe((searchState) => {
        if (this.currentState) {
          this.currentState.search = searchState;
          this.updateSearchUI(searchState);
        }
      });

      // Initialize app state
      await this.appState.initialize();

      // Hide loading screen and show app
      this.elements.loadingScreen.classList.add('hidden');
      this.elements.app.classList.remove('hidden');

      // Check for indexing progress
      this.checkIndexingProgress();

    } catch (error) {
      console.error('Failed to initialize app:', error);
      this.showError('Failed to initialize application: ' + handleError(error));
    }
  }

  /**
   * Check indexing progress
   */
  private async checkIndexingProgress(): Promise<void> {
    try {
      await this.appState.getProgress();
      const progress = this.currentState?.progress;
      if (progress && !progress.is_complete) {
        this.showProgressBar(progress);
        // Check again in 1 second
        setTimeout(() => this.checkIndexingProgress(), 1000);
      } else {
        this.hideProgressBar();
      }
    } catch (error) {
      console.error('Failed to check progress:', error);
    }
  }

  /**
   * Update UI based on app state
   */
  private updateUI(state: AppState): void {
    this.updateFileStats(state);
    this.updateProgressBar(state.progress);
    this.updateDarkMode(state.darkMode);
    this.updateExportButton(state.search.results.length > 0);
  }

  /**
   * Update search UI
   */
  private updateSearchUI(searchState: any): void {
    this.updateSearchInput(searchState);
    this.updateSearchResults(searchState);
    this.updatePagination(searchState);
    this.updateSearchStats(searchState);
  }

  /**
   * Update file statistics
   */
  private updateFileStats(state: AppState): void {
    if (state.stats) {
      const stats = state.stats;
      this.elements.fileStats.textContent = 
        `${formatNumber(stats.row_count)} rows, ${stats.column_count} columns, ${formatFileSize(stats.file_size_bytes)}`;
    } else {
      this.elements.fileStats.textContent = 'Loading...';
    }
  }

  /**
   * Update search input
   */
  private updateSearchInput(searchState: any): void {
    const hasQuery = searchState.query.trim().length > 0;
    this.elements.clearSearch.classList.toggle('hidden', !hasQuery);
    this.elements.searchLoading.classList.toggle('hidden', !searchState.loading);
  }

  /**
   * Update search results
   */
  private updateSearchResults(searchState: any): void {
    // Show/hide error message
    if (searchState.error) {
      this.elements.errorMessage.classList.remove('hidden');
      this.elements.errorText.textContent = searchState.error;
    } else {
      this.elements.errorMessage.classList.add('hidden');
    }

    // Show/hide loading state
    this.elements.loadingState.classList.toggle('hidden', !searchState.loading);

    // Show/hide results
    const hasResults = searchState.results.length > 0;
    this.elements.resultsHeader.classList.toggle('hidden', !hasResults);
    this.elements.resultsTable.classList.toggle('hidden', !hasResults);
    this.elements.emptyState.classList.toggle('hidden', hasResults || searchState.loading);

    if (hasResults) {
      this.renderResults(searchState.results, searchState.query);
    }
  }

  /**
   * Render search results
   */
  private renderResults(results: SearchResult[], query: string): void {
    if (results.length === 0) return;

    // Get columns from first result
    const columns = Object.keys(results[0].data);
    
    // Render header
    this.elements.tableHeader.innerHTML = '';
    columns.forEach(column => {
      const th = document.createElement('th');
      th.className = 'px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider';
      th.textContent = column;
      this.elements.tableHeader.appendChild(th);
    });

    // Render body
    this.elements.tableBody.innerHTML = '';
    results.forEach((result) => {
      const tr = document.createElement('tr');
      tr.className = 'hover:bg-gray-50 dark:hover:bg-gray-800';
      
      columns.forEach(column => {
        const td = document.createElement('td');
        td.className = 'px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white';
        
        const value = result.data[column] || '';
        const highlightedValue = query ? highlightText(value, query) : value;
        td.innerHTML = highlightedValue;
        
        tr.appendChild(td);
      });
      
      this.elements.tableBody.appendChild(tr);
    });
  }

  /**
   * Update pagination
   */
  private updatePagination(searchState: any): void {
    const hasResults = searchState.results.length > 0;
    this.elements.pagination.classList.toggle('hidden', !hasResults);

    if (hasResults) {
      this.elements.currentPage.textContent = searchState.page.toString();
      this.elements.totalPages.textContent = searchState.totalPages.toString();
      
      // Update pagination info
      const start = (searchState.page - 1) * searchState.limit + 1;
      const end = Math.min(searchState.page * searchState.limit, searchState.total);
      this.elements.paginationInfo.textContent = 
        `Showing ${start}-${end} of ${formatNumber(searchState.total)} results`;

      // Update prev/next buttons
      (this.elements.prevPage as HTMLButtonElement).disabled = searchState.page <= 1;
      (this.elements.nextPage as HTMLButtonElement).disabled = searchState.page >= searchState.totalPages;

      // Update page numbers
      this.renderPageNumbers(searchState.page, searchState.totalPages);
    }
  }

  /**
   * Render page numbers
   */
  private renderPageNumbers(currentPage: number, totalPages: number): void {
    this.elements.pageNumbers.innerHTML = '';
    
    const maxVisible = 5;
    let start = Math.max(1, currentPage - Math.floor(maxVisible / 2));
    let end = Math.min(totalPages, start + maxVisible - 1);
    
    if (end - start + 1 < maxVisible) {
      start = Math.max(1, end - maxVisible + 1);
    }

    for (let i = start; i <= end; i++) {
      const button = document.createElement('button');
      button.className = `px-3 py-2 text-sm font-medium border-t border-b border-gray-300 dark:border-gray-600 ${
        i === currentPage
          ? 'bg-blue-50 dark:bg-blue-900 text-blue-600 dark:text-blue-300 border-blue-500'
          : 'text-gray-500 dark:text-gray-400 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700'
      }`;
      button.textContent = i.toString();
      button.addEventListener('click', () => this.searchManager.setPage(i));
      this.elements.pageNumbers.appendChild(button);
    }
  }

  /**
   * Update search statistics
   */
  private updateSearchStats(searchState: any): void {
    const hasResults = searchState.results.length > 0;
    this.elements.searchStats.classList.toggle('hidden', !hasResults);

    if (hasResults) {
      this.elements.resultCount.textContent = formatNumber(searchState.total);
      this.elements.searchDuration.textContent = formatDuration(searchState.duration);
    }
  }

  /**
   * Update suggestions
   */
  private async updateSuggestions(query: string): Promise<void> {
    if (query.trim().length < 2) {
      this.hideSuggestions();
      return;
    }

    try {
      await this.appState.getSuggestions(query);
      this.showSuggestions(this.currentState?.suggestions || []);
    } catch (error) {
      console.error('Failed to get suggestions:', error);
    }
  }

  /**
   * Show suggestions
   */
  private showSuggestions(suggestions: string[]): void {
    if (suggestions.length === 0) {
      this.hideSuggestions();
      return;
    }

    this.elements.suggestionsList.innerHTML = '';
    suggestions.forEach(suggestion => {
      const div = document.createElement('div');
      div.className = 'px-4 py-2 hover:bg-gray-100 dark:hover:bg-gray-700 cursor-pointer text-sm';
      div.textContent = suggestion;
      div.addEventListener('click', () => {
        this.elements.searchInput.value = suggestion;
        this.searchManager.setQuery(suggestion);
        this.hideSuggestions();
      });
      this.elements.suggestionsList.appendChild(div);
    });

    this.elements.suggestions.classList.remove('hidden');
  }

  /**
   * Hide suggestions
   */
  private hideSuggestions(): void {
    this.elements.suggestions.classList.add('hidden');
  }

  /**
   * Update dark mode
   */
  private updateDarkMode(darkMode: boolean): void {
    if (darkMode) {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }

  /**
   * Update export button
   */
  private updateExportButton(enabled: boolean): void {
    (this.elements.exportBtn as HTMLButtonElement).disabled = !enabled;
  }

  /**
   * Show progress bar
   */
  private showProgressBar(progress: any): void {
    this.elements.progressBar.classList.remove('hidden');
    this.elements.progressFill.style.width = `${progress.progress_percent}%`;
  }

  /**
   * Hide progress bar
   */
  private hideProgressBar(): void {
    this.elements.progressBar.classList.add('hidden');
  }

  /**
   * Update progress bar
   */
  private updateProgressBar(progress: any): void {
    if (progress && !progress.is_complete) {
      this.showProgressBar(progress);
    } else {
      this.hideProgressBar();
    }
  }

  /**
   * Show error message
   */
  private showError(message: string): void {
    this.elements.errorMessage.classList.remove('hidden');
    this.elements.errorText.textContent = message;
  }

  /**
   * Export results
   */
  private exportResults(): void {
    if (!this.currentState?.search.results.length) return;

    try {
      const data = this.currentState.search.results.map(result => result.data);
      const filename = `csv-search-results-${new Date().toISOString().split('T')[0]}.csv`;
      downloadCSV(data, filename);
    } catch (error) {
      this.showError('Failed to export results: ' + handleError(error));
    }
  }
}

// Initialize app when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  new CSVSearchApp();
});
