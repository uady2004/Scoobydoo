import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/followers/presentation/providers/followers_provider.dart';
import 'package:tiktok_clone/features/followers/presentation/widgets/user_list_tile.dart';

class FollowersScreen extends ConsumerStatefulWidget {
  final String userId;

  const FollowersScreen({super.key, required this.userId});

  @override
  ConsumerState<FollowersScreen> createState() => _FollowersScreenState();
}

class _FollowersScreenState extends ConsumerState<FollowersScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  final _followersScroll = ScrollController();
  final _followingScroll = ScrollController();

  UserListArg get _followersArg =>
      (userId: widget.userId, isFollowers: true);
  UserListArg get _followingArg =>
      (userId: widget.userId, isFollowers: false);

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    _followersScroll.addListener(() => _onScroll(_followersScroll, true));
    _followingScroll.addListener(() => _onScroll(_followingScroll, false));
  }

  void _onScroll(ScrollController ctrl, bool isFollowers) {
    if (ctrl.position.pixels >= ctrl.position.maxScrollExtent - 300) {
      final arg = isFollowers ? _followersArg : _followingArg;
      ref.read(followersListProvider(arg).notifier).loadMore();
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    _followersScroll.dispose();
    _followingScroll.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        iconTheme: const IconThemeData(color: Colors.white),
        title: const Text(
          'Followers',
          style: TextStyle(color: Colors.white, fontWeight: FontWeight.w600),
        ),
        bottom: TabBar(
          controller: _tabController,
          indicatorColor: const Color(0xFFFF0050),
          labelColor: Colors.white,
          unselectedLabelColor: Colors.white38,
          tabs: const [
            Tab(text: 'Followers'),
            Tab(text: 'Following'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          _UserList(
            arg: _followersArg,
            scrollController: _followersScroll,
          ),
          _UserList(
            arg: _followingArg,
            scrollController: _followingScroll,
          ),
        ],
      ),
    );
  }
}

class _UserList extends ConsumerWidget {
  final UserListArg arg;
  final ScrollController scrollController;

  const _UserList({required this.arg, required this.scrollController});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncState = ref.watch(followersListProvider(arg));

    return asyncState.when(
      loading: () => const Center(
        child: CircularProgressIndicator(color: Color(0xFFFF0050)),
      ),
      error: (e, _) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline, color: Colors.white54, size: 48),
            const SizedBox(height: 12),
            Text(e.toString(),
                style: const TextStyle(color: Colors.white54),
                textAlign: TextAlign.center),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: () => ref.invalidate(followersListProvider(arg)),
              style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFF0050)),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (state) {
        if (state.users.isEmpty && !state.isLoading) {
          return Center(
            child: Text(
              arg.isFollowers ? 'No followers yet' : 'Not following anyone',
              style: const TextStyle(color: Colors.white54, fontSize: 15),
            ),
          );
        }

        return ListView.builder(
          controller: scrollController,
          itemCount: state.users.length + (state.isLoadingMore ? 1 : 0),
          itemBuilder: (context, index) {
            if (index == state.users.length) {
              return const Padding(
                padding: EdgeInsets.all(16),
                child: Center(
                  child: CircularProgressIndicator(
                    color: Color(0xFFFF0050),
                    strokeWidth: 2,
                  ),
                ),
              );
            }
            return UserListTile(user: state.users[index]);
          },
        );
      },
    );
  }
}
