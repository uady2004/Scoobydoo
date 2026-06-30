import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/followers/data/datasources/followers_remote_datasource.dart';

// ---------------------------------------------------------------------------
// Infrastructure
// ---------------------------------------------------------------------------

final followersRemoteDataSourceProvider = Provider<FollowersRemoteDataSource>(
  (_) => FollowersRemoteDataSourceImpl(ApiClient.instance.dio),
);

// ---------------------------------------------------------------------------
// Follow toggle — family keyed by targetUserId
// ---------------------------------------------------------------------------

class FollowState {
  final bool isFollowing;
  final bool isLoading;

  const FollowState({required this.isFollowing, this.isLoading = false});

  FollowState copyWith({bool? isFollowing, bool? isLoading}) => FollowState(
        isFollowing: isFollowing ?? this.isFollowing,
        isLoading: isLoading ?? this.isLoading,
      );
}

class FollowNotifier extends FamilyNotifier<FollowState, String> {
  @override
  FollowState build(String userId) =>
      const FollowState(isFollowing: false);

  Future<void> toggle() async {
    final previous = state;
    state = state.copyWith(
      isFollowing: !state.isFollowing,
      isLoading: true,
    );

    try {
      final ds = ref.read(followersRemoteDataSourceProvider);
      if (previous.isFollowing) {
        await ds.unfollowUser(arg);
        state = const FollowState(isFollowing: false);
      } else {
        final isFollowing = await ds.followUser(arg);
        state = FollowState(isFollowing: isFollowing);
      }
    } catch (_) {
      state = previous;
    }
  }
}

final followProvider =
    NotifierProviderFamily<FollowNotifier, FollowState, String>(
  FollowNotifier.new,
);

// ---------------------------------------------------------------------------
// Paginated user list state
// ---------------------------------------------------------------------------

class UserListState {
  final List<Map<String, dynamic>> users;
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final String? nextCursor;
  final String? error;

  const UserListState({
    this.users = const [],
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.nextCursor,
    this.error,
  });

  UserListState copyWith({
    List<Map<String, dynamic>>? users,
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    String? nextCursor,
    String? error,
  }) {
    return UserListState(
      users: users ?? this.users,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      error: error ?? this.error,
    );
  }
}

// ---------------------------------------------------------------------------
// Followers list notifier — family keyed by userId
// ---------------------------------------------------------------------------

typedef UserListArg = ({String userId, bool isFollowers});

class UserListNotifier extends FamilyAsyncNotifier<UserListState, UserListArg> {
  @override
  Future<UserListState> build(UserListArg arg) async {
    return _fetch();
  }

  Future<UserListState> _fetch({String? cursor}) async {
    final ds = ref.read(followersRemoteDataSourceProvider);
    final (items, nextCursor) = arg.isFollowers
        ? await ds.getFollowers(userId: arg.userId, cursor: cursor)
        : await ds.getFollowing(userId: arg.userId, cursor: cursor);

    return UserListState(
      users: items,
      nextCursor: nextCursor,
      hasMore: nextCursor != null,
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || !current.hasMore || current.isLoadingMore) return;

    state = AsyncValue.data(current.copyWith(isLoadingMore: true));
    try {
      final ds = ref.read(followersRemoteDataSourceProvider);
      final (items, nextCursor) = arg.isFollowers
          ? await ds.getFollowers(
              userId: arg.userId, cursor: current.nextCursor)
          : await ds.getFollowing(
              userId: arg.userId, cursor: current.nextCursor);

      state = AsyncValue.data(current.copyWith(
        users: [...current.users, ...items],
        nextCursor: nextCursor,
        hasMore: nextCursor != null,
        isLoadingMore: false,
      ));
    } catch (e) {
      state = AsyncValue.data(
          current.copyWith(isLoadingMore: false, error: e.toString()));
    }
  }
}

final followersListProvider =
    AsyncNotifierProviderFamily<UserListNotifier, UserListState, UserListArg>(
  UserListNotifier.new,
);
