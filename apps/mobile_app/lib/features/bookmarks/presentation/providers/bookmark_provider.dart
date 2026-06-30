import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/bookmarks/data/datasources/bookmark_remote_datasource.dart';
import 'package:tiktok_clone/core/network/api_client.dart';

// ---------------------------------------------------------------------------
// Infrastructure
// ---------------------------------------------------------------------------

final bookmarkRemoteDataSourceProvider = Provider<BookmarkRemoteDataSource>(
  (ref) => BookmarkRemoteDataSourceImpl(ApiClient.instance.dio),
);

// ---------------------------------------------------------------------------
// Bookmark toggle state — family keyed by videoId
// ---------------------------------------------------------------------------

class BookmarkState {
  final bool isBookmarked;
  final bool isLoading;

  const BookmarkState({required this.isBookmarked, this.isLoading = false});

  BookmarkState copyWith({bool? isBookmarked, bool? isLoading}) {
    return BookmarkState(
      isBookmarked: isBookmarked ?? this.isBookmarked,
      isLoading: isLoading ?? this.isLoading,
    );
  }
}

class BookmarkNotifier extends FamilyNotifier<BookmarkState, String> {
  @override
  BookmarkState build(String videoId) => const BookmarkState(isBookmarked: false);

  Future<void> toggle() async {
    final previous = state;
    state = state.copyWith(
      isBookmarked: !state.isBookmarked,
      isLoading: true,
    );

    try {
      final ds = ref.read(bookmarkRemoteDataSourceProvider);
      final serverValue = await ds.toggleBookmark(arg);
      state = state.copyWith(isBookmarked: serverValue, isLoading: false);
    } catch (_) {
      state = previous;
    }
  }
}

final bookmarkProvider =
    NotifierProviderFamily<BookmarkNotifier, BookmarkState, String>(
  BookmarkNotifier.new,
);

// ---------------------------------------------------------------------------
// Bookmarked videos list state
// ---------------------------------------------------------------------------

class BookmarkedVideosState {
  final List<Map<String, dynamic>> videos;
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final String? nextCursor;
  final String? error;

  const BookmarkedVideosState({
    this.videos = const [],
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.nextCursor,
    this.error,
  });

  BookmarkedVideosState copyWith({
    List<Map<String, dynamic>>? videos,
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    String? nextCursor,
    String? error,
  }) {
    return BookmarkedVideosState(
      videos: videos ?? this.videos,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      error: error ?? this.error,
    );
  }
}

class BookmarkedVideosNotifier extends AsyncNotifier<BookmarkedVideosState> {
  @override
  Future<BookmarkedVideosState> build() async {
    return _fetchPage();
  }

  Future<BookmarkedVideosState> _fetchPage({String? cursor}) async {
    final ds = ref.read(bookmarkRemoteDataSourceProvider);
    final (items, nextCursor) = await ds.getBookmarkedVideos(cursor: cursor);
    return BookmarkedVideosState(
      videos: items,
      nextCursor: nextCursor,
      hasMore: nextCursor != null,
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || !current.hasMore || current.isLoadingMore) return;

    state = AsyncValue.data(current.copyWith(isLoadingMore: true));
    try {
      final ds = ref.read(bookmarkRemoteDataSourceProvider);
      final (items, nextCursor) =
          await ds.getBookmarkedVideos(cursor: current.nextCursor);
      state = AsyncValue.data(current.copyWith(
        videos: [...current.videos, ...items],
        nextCursor: nextCursor,
        hasMore: nextCursor != null,
        isLoadingMore: false,
      ));
    } catch (e) {
      state = AsyncValue.data(
          current.copyWith(isLoadingMore: false, error: e.toString()));
    }
  }

  void removeVideo(String videoId) {
    final current = state.valueOrNull;
    if (current == null) return;
    state = AsyncValue.data(current.copyWith(
      videos: current.videos.where((v) => v['id'] != videoId).toList(),
    ));
    ref.read(bookmarkRemoteDataSourceProvider).toggleBookmark(videoId);
  }
}

final bookmarkedVideosProvider =
    AsyncNotifierProvider<BookmarkedVideosNotifier, BookmarkedVideosState>(
  BookmarkedVideosNotifier.new,
);
