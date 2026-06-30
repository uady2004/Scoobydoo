import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/search/data/datasources/search_remote_datasource.dart';

// ---------------------------------------------------------------------------
// Infrastructure
// ---------------------------------------------------------------------------

final searchRemoteDataSourceProvider = Provider<SearchRemoteDataSource>(
  (_) => SearchRemoteDataSourceImpl(ApiClient.instance.dio),
);

// ---------------------------------------------------------------------------
// Search filter type
// ---------------------------------------------------------------------------

enum SearchFilterType { all, videos, users, live }

extension SearchFilterTypeExt on SearchFilterType {
  String get label {
    switch (this) {
      case SearchFilterType.all:
        return 'All';
      case SearchFilterType.videos:
        return 'Videos';
      case SearchFilterType.users:
        return 'Users';
      case SearchFilterType.live:
        return 'LIVE';
    }
  }

  String? get apiValue {
    switch (this) {
      case SearchFilterType.all:
        return null;
      case SearchFilterType.videos:
        return 'video';
      case SearchFilterType.users:
        return 'user';
      case SearchFilterType.live:
        return 'live';
    }
  }
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

class SearchState {
  final String query;
  final List<Map<String, dynamic>> results;
  final List<String> suggestions;
  final List<String> history;
  final List<Map<String, dynamic>> trending;
  final SearchFilterType activeFilter;
  final bool isSearching;
  final bool isLoadingMore;
  final bool hasMore;
  final String? nextCursor;
  final String? error;

  const SearchState({
    this.query = '',
    this.results = const [],
    this.suggestions = const [],
    this.history = const [],
    this.trending = const [],
    this.activeFilter = SearchFilterType.all,
    this.isSearching = false,
    this.isLoadingMore = false,
    this.hasMore = false,
    this.nextCursor,
    this.error,
  });

  bool get hasQuery => query.trim().isNotEmpty;

  SearchState copyWith({
    String? query,
    List<Map<String, dynamic>>? results,
    List<String>? suggestions,
    List<String>? history,
    List<Map<String, dynamic>>? trending,
    SearchFilterType? activeFilter,
    bool? isSearching,
    bool? isLoadingMore,
    bool? hasMore,
    String? nextCursor,
    String? error,
    bool clearError = false,
  }) {
    return SearchState(
      query: query ?? this.query,
      results: results ?? this.results,
      suggestions: suggestions ?? this.suggestions,
      history: history ?? this.history,
      trending: trending ?? this.trending,
      activeFilter: activeFilter ?? this.activeFilter,
      isSearching: isSearching ?? this.isSearching,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      error: clearError ? null : error ?? this.error,
    );
  }
}

// ---------------------------------------------------------------------------
// Notifier — StateNotifier with 400 ms debounce on query changes
// ---------------------------------------------------------------------------

class SearchNotifier extends StateNotifier<SearchState> {
  final SearchRemoteDataSource _ds;
  Timer? _debounce;

  SearchNotifier(this._ds) : super(const SearchState()) {
    _loadInitial();
  }

  Future<void> _loadInitial() async {
    try {
      final results = await Future.wait([
        _ds.getTrendingSearches(),
        _ds.getSearchHistory(),
      ]);
      state = state.copyWith(
        trending: results[0] as List<Map<String, dynamic>>,
        history: results[1] as List<String>,
      );
    } catch (_) {}
  }

  void onQueryChanged(String q) {
    state = state.copyWith(query: q, clearError: true);

    _debounce?.cancel();

    if (q.trim().isEmpty) {
      state = state.copyWith(suggestions: [], results: []);
      return;
    }

    // Suggestions appear immediately after debounce
    _debounce = Timer(const Duration(milliseconds: 400), () async {
      await _fetchSuggestions(q.trim());
    });
  }

  Future<void> _fetchSuggestions(String q) async {
    try {
      final suggestions = await _ds.getSuggestions(q);
      state = state.copyWith(suggestions: suggestions);
    } catch (_) {}
  }

  Future<void> search({String? query, SearchFilterType? filter}) async {
    final q = (query ?? state.query).trim();
    if (q.isEmpty) return;

    final activeFilter = filter ?? state.activeFilter;
    state = state.copyWith(
      query: q,
      activeFilter: activeFilter,
      isSearching: true,
      results: [],
      suggestions: [],
      clearError: true,
    );

    try {
      await _ds.saveSearchHistory(q);
      final (items, nextCursor) = await _ds.searchAll(
        q: q,
        type: activeFilter.apiValue,
      );
      // Refresh history in background
      _ds.getSearchHistory().then((h) {
        state = state.copyWith(history: h);
      }).ignore();

      state = state.copyWith(
        results: items,
        nextCursor: nextCursor,
        hasMore: nextCursor != null,
        isSearching: false,
      );
    } catch (e) {
      state = state.copyWith(isSearching: false, error: e.toString());
    }
  }

  Future<void> loadMore() async {
    if (!state.hasMore || state.isLoadingMore || state.query.isEmpty) return;

    state = state.copyWith(isLoadingMore: true);
    try {
      final (items, nextCursor) = await _ds.searchAll(
        q: state.query,
        type: state.activeFilter.apiValue,
        cursor: state.nextCursor,
      );
      state = state.copyWith(
        results: [...state.results, ...items],
        nextCursor: nextCursor,
        hasMore: nextCursor != null,
        isLoadingMore: false,
      );
    } catch (e) {
      state = state.copyWith(isLoadingMore: false, error: e.toString());
    }
  }

  void setFilter(SearchFilterType filter) {
    if (filter == state.activeFilter) return;
    search(filter: filter);
  }

  Future<void> clearHistory() async {
    try {
      await _ds.clearHistory();
      state = state.copyWith(history: []);
    } catch (_) {}
  }

  void clearQuery() {
    _debounce?.cancel();
    state = state.copyWith(
      query: '',
      results: [],
      suggestions: [],
      clearError: true,
    );
  }

  @override
  void dispose() {
    _debounce?.cancel();
    super.dispose();
  }
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

final searchProvider =
    StateNotifierProvider<SearchNotifier, SearchState>(
  (ref) => SearchNotifier(ref.watch(searchRemoteDataSourceProvider)),
);
