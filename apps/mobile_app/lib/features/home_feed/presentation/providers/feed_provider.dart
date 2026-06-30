import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/datasources/feed_remote_datasource.dart';
import '../../data/repositories/feed_repository_impl.dart';
import '../../domain/entities/feed_item_entity.dart';
import '../../domain/repositories/feed_repository.dart';
import '../../domain/usecases/get_for_you_feed_usecase.dart';
import '../../domain/usecases/get_following_feed_usecase.dart';
import '../../domain/usecases/report_view_usecase.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Dependency providers
// ─────────────────────────────────────────────────────────────────────────────

final feedRemoteDataSourceProvider = Provider<FeedRemoteDataSource>((ref) {
  return FeedRemoteDataSourceImpl();
});

final feedRepositoryProvider = Provider<FeedRepository>((ref) {
  return FeedRepositoryImpl(ref.read(feedRemoteDataSourceProvider));
});

final getForYouFeedUseCaseProvider = Provider<GetForYouFeedUseCase>((ref) {
  return GetForYouFeedUseCase(ref.read(feedRepositoryProvider));
});

final getFollowingFeedUseCaseProvider = Provider<GetFollowingFeedUseCase>((ref) {
  return GetFollowingFeedUseCase(ref.read(feedRepositoryProvider));
});

final reportViewUseCaseProvider = Provider<ReportViewUseCase>((ref) {
  return ReportViewUseCase(ref.read(feedRepositoryProvider));
});

// ─────────────────────────────────────────────────────────────────────────────
// Feed state
// ─────────────────────────────────────────────────────────────────────────────

class FeedState {
  const FeedState({
    this.items = const [],
    this.nextCursor,
    this.isLoadingMore = false,
    this.hasReachedEnd = false,
    this.error,
  });

  final List<FeedItemEntity> items;
  final String? nextCursor;
  final bool isLoadingMore;
  final bool hasReachedEnd;

  /// Non-null when a load-more operation failed.
  final String? error;

  FeedState copyWith({
    List<FeedItemEntity>? items,
    String? nextCursor,
    bool? isLoadingMore,
    bool? hasReachedEnd,
    String? error,
    bool clearError = false,
  }) {
    return FeedState(
      items: items ?? this.items,
      nextCursor: nextCursor ?? this.nextCursor,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasReachedEnd: hasReachedEnd ?? this.hasReachedEnd,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// For You Feed notifier
// ─────────────────────────────────────────────────────────────────────────────

/// Manages paginated For You feed with pre-fetch buffer.
///
/// [AsyncNotifier] state represents the initial load; pagination is tracked
/// separately in [FeedState] so the PageView never blanks on load-more.
class ForYouFeedNotifier extends AsyncNotifier<FeedState> {
  static const int _prefetchThreshold = 3;

  @override
  Future<FeedState> build() async {
    return _fetchPage(cursor: null, existing: []);
  }

  /// Re-fetches from the beginning.
  Future<void> loadInitial() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _fetchPage(cursor: null, existing: []),
    );
  }

  /// Loads the next page. Safe to call multiple times — debounced via
  /// [isLoadingMore] flag. Automatically triggered when the user reaches
  /// 3 items from the end of the current list.
  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null) return;
    if (current.isLoadingMore || current.hasReachedEnd) return;

    state = AsyncData(current.copyWith(isLoadingMore: true, clearError: true));

    final useCase = ref.read(getForYouFeedUseCaseProvider);
    final result = await useCase(FeedParams(cursor: current.nextCursor));

    result.fold(
      (failure) {
        state = AsyncData(
          current.copyWith(isLoadingMore: false, error: failure.message),
        );
      },
      (page) {
        final updated = current.copyWith(
          items: [...current.items, ...page.items],
          nextCursor: page.nextCursor,
          isLoadingMore: false,
          hasReachedEnd: page.nextCursor == null,
          clearError: true,
        );
        state = AsyncData(updated);
      },
    );
  }

  /// Called by the PageView on every page-change event.
  /// Triggers [loadMore] when the user is [_prefetchThreshold] items
  /// from the last buffered item.
  void onPageChanged(int index) {
    final current = state.valueOrNull;
    if (current == null) return;
    if (index >= current.items.length - _prefetchThreshold) {
      loadMore();
    }
  }

  /// Optimistically updates the like state for [videoId].
  void toggleLike(String videoId) {
    final current = state.valueOrNull;
    if (current == null) return;
    final updated = current.items.map((item) {
      if (item.videoId != videoId) return item;
      return item.copyWith(
        isLiked: !item.isLiked,
        likeCount: item.isLiked ? item.likeCount - 1 : item.likeCount + 1,
      );
    }).toList();
    state = AsyncData(current.copyWith(items: updated));
  }

  /// Optimistically updates the bookmark state for [videoId].
  void toggleBookmark(String videoId) {
    final current = state.valueOrNull;
    if (current == null) return;
    final updated = current.items.map((item) {
      if (item.videoId != videoId) return item;
      return item.copyWith(
        isBookmarked: !item.isBookmarked,
        bookmarkCount: item.isBookmarked
            ? item.bookmarkCount - 1
            : item.bookmarkCount + 1,
      );
    }).toList();
    state = AsyncData(current.copyWith(items: updated));
  }

  /// Optimistically updates the follow state for [creatorId].
  void toggleFollow(String creatorId) {
    final current = state.valueOrNull;
    if (current == null) return;
    final updated = current.items.map((item) {
      if (item.creatorId != creatorId) return item;
      return item.copyWith(isFollowing: !item.isFollowing);
    }).toList();
    state = AsyncData(current.copyWith(items: updated));
  }

  // ── Private helpers ───────────────────────────────────────────────────────

  Future<FeedState> _fetchPage({
    required String? cursor,
    required List<FeedItemEntity> existing,
  }) async {
    final useCase = ref.read(getForYouFeedUseCaseProvider);
    final result = await useCase(FeedParams(cursor: cursor));

    return result.fold(
      (failure) => throw Exception(failure.message),
      (page) => FeedState(
        items: [...existing, ...page.items],
        nextCursor: page.nextCursor,
        hasReachedEnd: page.nextCursor == null,
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Following Feed notifier
// ─────────────────────────────────────────────────────────────────────────────

class FollowingFeedNotifier extends AsyncNotifier<FeedState> {
  static const int _prefetchThreshold = 3;

  @override
  Future<FeedState> build() async {
    return _fetchPage(cursor: null, existing: []);
  }

  Future<void> loadInitial() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _fetchPage(cursor: null, existing: []),
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null) return;
    if (current.isLoadingMore || current.hasReachedEnd) return;

    state = AsyncData(current.copyWith(isLoadingMore: true, clearError: true));

    final useCase = ref.read(getFollowingFeedUseCaseProvider);
    final result = await useCase(FeedParams(cursor: current.nextCursor));

    result.fold(
      (failure) {
        state = AsyncData(
          current.copyWith(isLoadingMore: false, error: failure.message),
        );
      },
      (page) {
        final updated = current.copyWith(
          items: [...current.items, ...page.items],
          nextCursor: page.nextCursor,
          isLoadingMore: false,
          hasReachedEnd: page.nextCursor == null,
          clearError: true,
        );
        state = AsyncData(updated);
      },
    );
  }

  void onPageChanged(int index) {
    final current = state.valueOrNull;
    if (current == null) return;
    if (index >= current.items.length - _prefetchThreshold) {
      loadMore();
    }
  }

  void toggleLike(String videoId) {
    final current = state.valueOrNull;
    if (current == null) return;
    final updated = current.items.map((item) {
      if (item.videoId != videoId) return item;
      return item.copyWith(
        isLiked: !item.isLiked,
        likeCount: item.isLiked ? item.likeCount - 1 : item.likeCount + 1,
      );
    }).toList();
    state = AsyncData(current.copyWith(items: updated));
  }

  void toggleBookmark(String videoId) {
    final current = state.valueOrNull;
    if (current == null) return;
    final updated = current.items.map((item) {
      if (item.videoId != videoId) return item;
      return item.copyWith(
        isBookmarked: !item.isBookmarked,
        bookmarkCount: item.isBookmarked
            ? item.bookmarkCount - 1
            : item.bookmarkCount + 1,
      );
    }).toList();
    state = AsyncData(current.copyWith(items: updated));
  }

  void toggleFollow(String creatorId) {
    final current = state.valueOrNull;
    if (current == null) return;
    final updated = current.items.map((item) {
      if (item.creatorId != creatorId) return item;
      return item.copyWith(isFollowing: !item.isFollowing);
    }).toList();
    state = AsyncData(current.copyWith(items: updated));
  }

  Future<FeedState> _fetchPage({
    required String? cursor,
    required List<FeedItemEntity> existing,
  }) async {
    final useCase = ref.read(getFollowingFeedUseCaseProvider);
    final result = await useCase(FeedParams(cursor: cursor));

    return result.fold(
      (failure) => throw Exception(failure.message),
      (page) => FeedState(
        items: [...existing, ...page.items],
        nextCursor: page.nextCursor,
        hasReachedEnd: page.nextCursor == null,
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Public provider references
// ─────────────────────────────────────────────────────────────────────────────

final forYouFeedProvider =
    AsyncNotifierProvider<ForYouFeedNotifier, FeedState>(
  ForYouFeedNotifier.new,
);

final followingFeedProvider =
    AsyncNotifierProvider<FollowingFeedNotifier, FeedState>(
  FollowingFeedNotifier.new,
);
