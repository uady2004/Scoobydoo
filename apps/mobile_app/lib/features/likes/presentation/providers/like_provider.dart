import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/likes/data/datasources/like_remote_datasource.dart';
import 'package:tiktok_clone/core/network/api_client.dart';

// ---------------------------------------------------------------------------
// Infrastructure
// ---------------------------------------------------------------------------

final likeRemoteDataSourceProvider = Provider<LikeRemoteDataSource>(
  (ref) => LikeRemoteDataSourceImpl(ApiClient.instance.dio),
);

// ---------------------------------------------------------------------------
// State per video
// ---------------------------------------------------------------------------

class LikeState {
  final bool isLiked;
  final int likeCount;
  final bool isLoading;

  const LikeState({
    required this.isLiked,
    required this.likeCount,
    this.isLoading = false,
  });

  LikeState copyWith({bool? isLiked, int? likeCount, bool? isLoading}) {
    return LikeState(
      isLiked: isLiked ?? this.isLiked,
      likeCount: likeCount ?? this.likeCount,
      isLoading: isLoading ?? this.isLoading,
    );
  }
}

// ---------------------------------------------------------------------------
// Notifier — family keyed by (videoId, initialLikeCount, initialIsLiked)
// We use a Record as the arg for the family so callers can seed initial state.
// ---------------------------------------------------------------------------

typedef LikeArg = ({String videoId, int initialCount, bool initialIsLiked});

class LikeNotifier extends FamilyNotifier<LikeState, LikeArg> {
  @override
  LikeState build(LikeArg arg) {
    return LikeState(
      isLiked: arg.initialIsLiked,
      likeCount: arg.initialCount,
    );
  }

  Future<void> toggle() async {
    final previous = state;

    // Optimistic update
    state = state.copyWith(
      isLiked: !state.isLiked,
      likeCount: state.isLiked ? state.likeCount - 1 : state.likeCount + 1,
      isLoading: true,
    );

    try {
      final ds = ref.read(likeRemoteDataSourceProvider);
      final serverIsLiked = await ds.toggleLike(arg.videoId);

      // Reconcile with server truth — count stays optimistic, just fix the flag
      state = state.copyWith(isLiked: serverIsLiked, isLoading: false);
    } catch (_) {
      // Roll back
      state = previous;
    }
  }
}

final likeProvider = NotifierProviderFamily<LikeNotifier, LikeState, LikeArg>(
  LikeNotifier.new,
);
