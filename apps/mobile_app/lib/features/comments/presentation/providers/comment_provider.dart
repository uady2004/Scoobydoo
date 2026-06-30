import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';
import 'package:tiktok_clone/features/comments/domain/usecases/create_comment_usecase.dart';
import 'package:tiktok_clone/features/comments/domain/usecases/get_comments_usecase.dart';
import 'package:tiktok_clone/features/comments/data/datasources/comment_remote_datasource.dart';
import 'package:tiktok_clone/features/comments/data/repositories/comment_repository_impl.dart';
import 'package:tiktok_clone/core/network/api_client.dart';

// ---------------------------------------------------------------------------
// Infrastructure providers
// ---------------------------------------------------------------------------

final commentRemoteDataSourceProvider = Provider<CommentRemoteDataSource>(
  (ref) => CommentRemoteDataSourceImpl(ApiClient.instance.dio),
);

final commentRepositoryProvider = Provider(
  (ref) => CommentRepositoryImpl(ref.watch(commentRemoteDataSourceProvider)),
);

final getCommentsUseCaseProvider = Provider(
  (ref) => GetCommentsUseCase(ref.watch(commentRepositoryProvider)),
);

final createCommentUseCaseProvider = Provider(
  (ref) => CreateCommentUseCase(ref.watch(commentRepositoryProvider)),
);

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

class CommentState {
  final List<CommentEntity> comments;
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final String? nextCursor;
  final String? error;

  const CommentState({
    this.comments = const [],
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.nextCursor,
    this.error,
  });

  CommentState copyWith({
    List<CommentEntity>? comments,
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    String? nextCursor,
    String? error,
    bool clearError = false,
  }) {
    return CommentState(
      comments: comments ?? this.comments,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      error: clearError ? null : error ?? this.error,
    );
  }
}

// ---------------------------------------------------------------------------
// Notifier — extends FamilyAsyncNotifier so it can be used with .family
// The family arg is the videoId.
// ---------------------------------------------------------------------------

class CommentNotifier extends FamilyAsyncNotifier<CommentState, String> {
  // Populated from arg in build(); available throughout the notifier lifetime.
  late final String _videoId;

  @override
  Future<CommentState> build(String arg) async {
    _videoId = arg;
    return _fetchFirstPage();
  }

  Future<CommentState> _fetchFirstPage() async {
    final useCase = ref.read(getCommentsUseCaseProvider);
    final result = await useCase(GetCommentsParams(videoId: _videoId));
    return result.fold(
      (failure) => CommentState(error: failure.message),
      (data) {
        final (comments, cursor) = data;
        final sorted = [...comments]..sort((a, b) {
            if (a.isPinned && !b.isPinned) return -1;
            if (!a.isPinned && b.isPinned) return 1;
            return b.createdAt.compareTo(a.createdAt);
          });
        return CommentState(
          comments: sorted,
          nextCursor: cursor,
          hasMore: cursor != null,
        );
      },
    );
  }

  Future<void> loadComments() async {
    state = const AsyncValue.loading();

    final useCase = ref.read(getCommentsUseCaseProvider);
    final result = await useCase(GetCommentsParams(videoId: _videoId));

    result.fold(
      (failure) => state = AsyncValue.data(CommentState(error: failure.message)),
      (data) {
        final (comments, cursor) = data;
        // Sort: pinned comments first
        final sorted = [...comments]..sort((a, b) {
            if (a.isPinned && !b.isPinned) return -1;
            if (!a.isPinned && b.isPinned) return 1;
            return b.createdAt.compareTo(a.createdAt);
          });
        state = AsyncValue.data(CommentState(
          comments: sorted,
          nextCursor: cursor,
          hasMore: cursor != null,
        ));
      },
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || !current.hasMore || current.isLoadingMore) return;

    state = AsyncValue.data(current.copyWith(isLoadingMore: true));

    final useCase = ref.read(getCommentsUseCaseProvider);
    final result = await useCase(
      GetCommentsParams(videoId: _videoId, cursor: current.nextCursor),
    );

    result.fold(
      (failure) => state = AsyncValue.data(
        current.copyWith(isLoadingMore: false, error: failure.message),
      ),
      (data) {
        final (newComments, cursor) = data;
        final updated = current.copyWith(
          comments: [...current.comments, ...newComments],
          nextCursor: cursor,
          hasMore: cursor != null,
          isLoadingMore: false,
        );
        state = AsyncValue.data(updated);
      },
    );
  }

  Future<void> createComment({
    required String content,
    String? parentId,
  }) async {
    final current = state.valueOrNull;
    if (current == null) return;

    // Optimistic: build a placeholder comment
    final optimisticId = 'optimistic_${DateTime.now().millisecondsSinceEpoch}';
    final optimistic = CommentEntity(
      id: optimisticId,
      videoId: _videoId,
      userId: 'me',
      username: 'you',
      avatarUrl: '',
      content: content,
      likeCount: 0,
      isLiked: false,
      isPinned: false,
      replyCount: 0,
      parentId: parentId,
      createdAt: DateTime.now(),
    );

    // Prepend optimistic comment
    state = AsyncValue.data(
      current.copyWith(comments: [optimistic, ...current.comments]),
    );

    final useCase = ref.read(createCommentUseCaseProvider);
    final result = await useCase(CreateCommentParams(
      videoId: _videoId,
      content: content,
      parentId: parentId,
    ));

    result.fold(
      (failure) {
        // Roll back optimistic
        final rolled = state.valueOrNull;
        if (rolled != null) {
          state = AsyncValue.data(
            rolled.copyWith(
              comments: rolled.comments.where((c) => c.id != optimisticId).toList(),
              error: failure.message,
            ),
          );
        }
      },
      (confirmed) {
        // Replace optimistic with confirmed
        final rolled = state.valueOrNull;
        if (rolled != null) {
          final updated = rolled.comments
              .map((c) => c.id == optimisticId ? confirmed : c)
              .toList();
          state = AsyncValue.data(rolled.copyWith(comments: updated));
        }
      },
    );
  }

  Future<void> toggleLike(String commentId) async {
    final current = state.valueOrNull;
    if (current == null) return;

    // Optimistic toggle
    final updated = current.comments.map((c) {
      if (c.id != commentId) return c;
      return c.copyWith(
        isLiked: !c.isLiked,
        likeCount: c.isLiked ? c.likeCount - 1 : c.likeCount + 1,
      );
    }).toList();

    state = AsyncValue.data(current.copyWith(comments: updated));

    final repo = ref.read(commentRepositoryProvider);
    final result = await repo.likeComment(commentId);

    result.fold(
      (failure) {
        // Roll back
        final rolled = state.valueOrNull;
        if (rolled != null) {
          final reverted = rolled.comments.map((c) {
            if (c.id != commentId) return c;
            return c.copyWith(
              isLiked: !c.isLiked,
              likeCount: c.isLiked ? c.likeCount - 1 : c.likeCount + 1,
            );
          }).toList();
          state = AsyncValue.data(rolled.copyWith(comments: reverted));
        }
      },
      (serverIsLiked) {
        // Reconcile with server truth
        final rolled = state.valueOrNull;
        if (rolled != null) {
          final reconciled = rolled.comments.map((c) {
            if (c.id != commentId) return c;
            // Count was already toggled optimistically; adjust only if mismatch
            return c.copyWith(isLiked: serverIsLiked);
          }).toList();
          state = AsyncValue.data(rolled.copyWith(comments: reconciled));
        }
      },
    );
  }

  Future<void> deleteComment(String commentId) async {
    final current = state.valueOrNull;
    if (current == null) return;

    final repo = ref.read(commentRepositoryProvider);
    final result = await repo.deleteComment(commentId);

    result.fold(
      (failure) {
        final s = state.valueOrNull;
        if (s != null) state = AsyncValue.data(s.copyWith(error: failure.message));
      },
      (_) {
        final s = state.valueOrNull;
        if (s != null) {
          state = AsyncValue.data(
            s.copyWith(
              comments: s.comments.where((c) => c.id != commentId).toList(),
            ),
          );
        }
      },
    );
  }

  Future<void> pinComment(String commentId) async {
    final repo = ref.read(commentRepositoryProvider);
    final result = await repo.pinComment(commentId);
    result.fold(
      (failure) {},
      (_) {
        final s = state.valueOrNull;
        if (s != null) {
          final updated = s.comments.map((c) {
            if (c.id == commentId) return c.copyWith(isPinned: true);
            // Unpin any other pinned comment
            if (c.isPinned) return c.copyWith(isPinned: false);
            return c;
          }).toList();
          // Re-sort pinned to top
          updated.sort((a, b) {
            if (a.isPinned && !b.isPinned) return -1;
            if (!a.isPinned && b.isPinned) return 1;
            return b.createdAt.compareTo(a.createdAt);
          });
          state = AsyncValue.data(s.copyWith(comments: updated));
        }
      },
    );
  }
}

// ---------------------------------------------------------------------------
// Provider family — keyed by videoId
// ---------------------------------------------------------------------------

final commentProvider =
    AsyncNotifierProviderFamily<CommentNotifier, CommentState, String>(
  CommentNotifier.new,
);
