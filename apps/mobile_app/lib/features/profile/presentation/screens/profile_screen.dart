import 'dart:io';
import 'package:go_router/go_router.dart';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:share_plus/share_plus.dart';
import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/follow_user_usecase.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/get_user_videos_usecase.dart';
import 'package:tiktok_clone/features/profile/presentation/providers/profile_provider.dart';
import 'package:tiktok_clone/features/profile/presentation/widgets/profile_stats_row.dart';
import 'package:tiktok_clone/features/profile/presentation/widgets/video_grid_tab.dart';
import 'package:tiktok_clone/features/messaging/presentation/providers/messaging_provider.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Sliver persistent header delegate for the sticky TabBar
// ─────────────────────────────────────────────────────────────────────────────

class _StickyTabBarDelegate extends SliverPersistentHeaderDelegate {
  const _StickyTabBarDelegate(this._tabBar);

  final TabBar _tabBar;

  @override
  double get minExtent => _tabBar.preferredSize.height;

  @override
  double get maxExtent => _tabBar.preferredSize.height;

  @override
  Widget build(
    BuildContext context,
    double shrinkOffset,
    bool overlapsContent,
  ) {
    return ColoredBox(
      color: Theme.of(context).scaffoldBackgroundColor,
      child: _tabBar,
    );
  }

  @override
  bool shouldRebuild(_StickyTabBarDelegate old) => false;
}

// ─────────────────────────────────────────────────────────────────────────────
// Profile screen
// ─────────────────────────────────────────────────────────────────────────────

class ProfileScreen extends ConsumerStatefulWidget {
  /// Pass [userId] to view another user's profile.
  /// Omit (null) to show the signed-in user's own profile.
  const ProfileScreen({super.key, this.userId});

  final String? userId;

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen>
    with TickerProviderStateMixin {
  late TabController _tabController;
  bool _bioExpanded = false;
  String? _resolvedUserId;
  bool _isOwnProfile = false;

  @override
  void initState() {
    super.initState();
    // 3 tabs for own profile; 2 for others (no bookmarks tab).
    _tabController = TabController(length: 3, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  // ── Helpers ────────────────────────────────────────────────────────────────

  Future<void> _openUrl(String raw) async {
    final url = raw.startsWith('http') ? raw : 'https://$raw';
    if (Platform.isAndroid) {
      await Process.run('am', [
        'start',
        '--user',
        '0',
        '-a',
        'android.intent.action.VIEW',
        '-d',
        url,
      ]);
    } else if (Platform.isIOS) {
      await Process.run('open', [url]);
    }
  }

  void _shareProfile(ProfileEntity profile) {
    SharePlus.instance.share(
      ShareParams(
        text: 'Check out @${profile.username} on TikTok Clone!\n'
            'https://tiktokclone.com/@${profile.username}',
        subject: '@${profile.username}',
      ),
    );
  }

  Future<void> _toggleFollow(String userId) async {
    final isFollowing = ref.read(followStateProvider(userId));
    // Optimistic toggle.
    ref.read(followStateProvider(userId).notifier).state = !isFollowing;

    final result = await ref.read(followUserUseCaseProvider).call(
          FollowUserParams(userId: userId, isFollowing: isFollowing),
        );

    result.fold(
      (failure) {
        // Roll back.
        ref.read(followStateProvider(userId).notifier).state = isFollowing;
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(failure.message)),
          );
        }
      },
      (_) {
        // Update follower count in the cached profile.
        ref.read(profileProvider(userId)).whenData((profile) {
          final delta = isFollowing ? -1 : 1;
          ref.read(profileProvider(userId).notifier).applyLocalUpdate(
                profile.copyWith(followerCount: profile.followerCount + delta),
              );
        });
      },
    );
  }

  // ── Sub-widgets ────────────────────────────────────────────────────────────

  Widget _buildAvatar(ProfileEntity profile) {
    return Stack(
      alignment: Alignment.bottomRight,
      children: [
        CircleAvatar(
          radius: 44,
          backgroundColor: Colors.grey.shade800,
          backgroundImage: profile.avatarUrl != null
              ? CachedNetworkImageProvider(profile.avatarUrl!)
              : null,
          child: profile.avatarUrl == null
              ? const Icon(Icons.person_rounded,
                  size: 44, color: Colors.white38)
              : null,
        ),
        if (profile.isVerified)
          Positioned(
            bottom: 0,
            right: 0,
            child: Container(
              padding: const EdgeInsets.all(2),
              decoration: const BoxDecoration(
                color: Color(0xFF20D5EC),
                shape: BoxShape.circle,
              ),
              child: const Icon(Icons.check_rounded,
                  color: Colors.white, size: 12),
            ),
          ),
      ],
    );
  }

  Widget _buildOwnProfileActions(ProfileEntity profile) {
    return Row(
      children: [
        Expanded(
          child: _OutlinedActionButton(
            label: 'Edit profile',
            onTap: () {
              context.push('/edit-profile', extra: profile).then((_) {
                if (_resolvedUserId != null) {
                  ref
                      .read(profileProvider(_resolvedUserId!).notifier)
                      .refresh();
                }
              });
            },
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: _OutlinedActionButton(
            label: 'Creator tools',
            onTap: () => context.push('/dashboard'),
          ),
        ),
      ],
    );
  }

  Widget _buildOtherProfileActions(String userId) {
    final isFollowing = ref.watch(followStateProvider(userId));

    return Row(
      children: [
        // Follow / Following — gradient when not yet following.
        Expanded(
          flex: 3,
          child: isFollowing
              ? _OutlinedActionButton(
                  label: 'Following',
                  onTap: () => _toggleFollow(userId),
                )
              : _GradientFollowButton(
                  onTap: () => _toggleFollow(userId),
                ),
        ),
        const SizedBox(width: 8),
        Expanded(
          flex: 2,
          child: _OutlinedActionButton(
            label: 'Message',
            onTap: () {
              // Find existing conversation with this user from inbox
              final conversations =
                  ref.read(inboxProvider).valueOrNull ?? [];
              final currentUserId = ref.read(currentUserIdProvider);

              // Find conversation where other participant matches userId
              final existing = conversations.cast<dynamic>().firstWhere(
                    (c) => c.participants.any(
                        (p) => p['id'] == userId || p['username'] == userId),
                    orElse: () => null,
                  );

              if (existing != null) {
                // Existing conversation found — open it directly
                context.go('/inbox');
                Future.delayed(const Duration(milliseconds: 300), () {
                  context.push(
                    '/inbox/chat/${existing.id}',
                    extra: {
                      'displayName': existing.displayName(currentUserId),
                      'avatarUrl': existing.avatarUrl(currentUserId),
                      'isOnline': existing.isOnline(currentUserId),
                    },
                  );
                });
              } else {
                // No existing conversation — go to inbox
                // In production: create conversation via API then navigate
                context.go('/inbox');
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(
                    content: Text('Starting a new conversation...'),
                    backgroundColor: Color(0xFF1A1A1A),
                    behavior: SnackBarBehavior.floating,
                  ),
                );
              }
            },
          ),
        ),
        const SizedBox(width: 8),
        // More options icon-button.
        _OutlinedIconButton(
          icon: Icons.more_horiz_rounded,
          onTap: () => _showMoreSheet(userId),
        ),
      ],
    );
  }

  void _showMoreSheet(String userId) {
    showModalBottomSheet<void>(
      context: context,
      builder: (_) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.block_rounded,
                  color: Color(0xFFEE1D52)),
              title: const Text('Block',
                  style: TextStyle(color: Color(0xFFEE1D52))),
              onTap: () {
                Navigator.pop(context);
                showDialog<void>(
                  context: context,
                  builder: (dialogContext) => AlertDialog(
                    backgroundColor: const Color(0xFF1A1A1A),
                    title: const Text('Block user',
                        style: TextStyle(color: Colors.white)),
                    content: const Text(
                      'They won\'t be able to see your content or contact you. They won\'t be notified.',
                      style: TextStyle(color: Colors.white70),
                    ),
                    actions: [
                      TextButton(
                        onPressed: () => Navigator.pop(dialogContext),
                        child: const Text('Cancel',
                            style: TextStyle(color: Colors.white54)),
                      ),
                      TextButton(
                        onPressed: () {
                          Navigator.pop(dialogContext);
                          context.pop(); // go back
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(
                              content: Text('User blocked'),
                              backgroundColor: Color(0xFF1A1A1A),
                              behavior: SnackBarBehavior.floating,
                            ),
                          );
                        },
                        child: const Text('Block',
                            style:
                                TextStyle(color: Color(0xFFEE1D52))),
                      ),
                    ],
                  ),
                );
              },
            ),
            ListTile(
              leading: const Icon(Icons.flag_outlined),
              title: const Text('Report'),
              onTap: () {
                Navigator.pop(context);
                _showReportSheet(context, userId);
              },
            ),
          ],
        ),
      ),
    );
  }
  void _showReportSheet(BuildContext context, String userId) {
    final reasons = [
      'Spam or fake account',
      'Inappropriate content',
      'Hate speech or symbols',
      'Harassment or bullying',
      'Violence or dangerous acts',
      'Intellectual property violation',
      'Other',
    ];

    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => SafeArea(
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: 12),
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(height: 16),
            const Text(
              'Report',
              style: TextStyle(
                color: Colors.white,
                fontSize: 16,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 8),
            const Divider(color: Colors.white12),
            ...reasons.map((reason) => ListTile(
                  title: Text(reason,
                      style: const TextStyle(
                          color: Colors.white, fontSize: 14)),
                  trailing: const Icon(Icons.chevron_right,
                      color: Colors.white24),
                  onTap: () {
                    Navigator.pop(context);
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text('Reported for: $reason'),
                        backgroundColor: const Color(0xFF1A1A1A),
                        behavior: SnackBarBehavior.floating,
                      ),
                    );
                  },
                )),
            const SizedBox(height: 8),
          ],
        ),
        ),
      ),
    );
  }

  Widget _buildProfileHeader(ProfileEntity profile, String userId) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          _buildAvatar(profile),
          const SizedBox(height: 10),
          // @username
          Text(
            '@${profile.username}',
            style: const TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w700,
              letterSpacing: -0.2,
            ),
          ),
          // Display name (only if different from username)
          if (profile.displayName.isNotEmpty &&
              profile.displayName != profile.username) ...[
            const SizedBox(height: 2),
            Text(
              profile.displayName,
              style: TextStyle(
                color: Colors.white.withValues(alpha: 0.65),
                fontSize: 13,
              ),
            ),
          ],
          const SizedBox(height: 14),
          ProfileStatsRow(
            userId: userId,
            followerCount: profile.followerCount,
            followingCount: profile.followingCount,
            likeCount: profile.likeCount,
          ),
          const SizedBox(height: 14),
          // Bio — tappable to expand.
          if (profile.bio != null && profile.bio!.isNotEmpty)
            GestureDetector(
              onTap: () => setState(() => _bioExpanded = !_bioExpanded),
              child: Text(
                profile.bio!,
                textAlign: TextAlign.center,
                maxLines: _bioExpanded ? null : 2,
                overflow:
                    _bioExpanded ? TextOverflow.visible : TextOverflow.ellipsis,
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.85),
                  fontSize: 13,
                  height: 1.45,
                ),
              ),
            ),
          // Website link.
          if (profile.website != null && profile.website!.isNotEmpty) ...[
            const SizedBox(height: 6),
            GestureDetector(
              onTap: () => _openUrl(profile.website!),
              child: Text(
                profile.website!,
                style: const TextStyle(
                  color: Color(0xFF20D5EC),
                  fontSize: 13,
                  decoration: TextDecoration.underline,
                  decorationColor: Color(0xFF20D5EC),
                ),
              ),
            ),
          ],
          const SizedBox(height: 14),
          if (_isOwnProfile)
            _buildOwnProfileActions(profile)
          else
            _buildOtherProfileActions(userId),
          const SizedBox(height: 4),
        ],
      ),
    );
  }

  // ── Scaffold ───────────────────────────────────────────────────────────────

  Widget _buildScaffold(ProfileEntity profile, String userId) {
    final tabCount = _isOwnProfile ? 3 : 2;
    if (_tabController.length != tabCount) {
      _tabController.dispose();
      _tabController = TabController(length: tabCount, vsync: this);
    }

    final tabBar = TabBar(
      controller: _tabController,
      indicatorColor: Colors.white,
      indicatorWeight: 1.5,
      labelColor: Colors.white,
      unselectedLabelColor: Colors.white38,
      tabs: [
        const Tab(icon: Icon(Icons.grid_on_rounded, size: 22)),
        const Tab(icon: Icon(Icons.favorite_border_rounded, size: 22)),
        if (_isOwnProfile)
          const Tab(icon: Icon(Icons.bookmark_border_rounded, size: 22)),
      ],
    );

    return Scaffold(
      backgroundColor: Colors.black,
      body: NestedScrollView(
        headerSliverBuilder: (context, innerBoxIsScrolled) => [
          // App bar
          SliverAppBar(
            pinned: true,
            backgroundColor: Colors.black,
            elevation: 0,
            automaticallyImplyLeading: false,
            leading: !_isOwnProfile
                ? IconButton(
                    icon: const Icon(Icons.arrow_back_ios_new_rounded,
                        color: Colors.white),
                    onPressed: () => Navigator.pop(context),
                  )
                : null,
            title: Text(
              '@${profile.username}',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            centerTitle: true,
            actions: [
              IconButton(
                icon: const Icon(Icons.share_outlined, color: Colors.white),
                onPressed: () => _shareProfile(profile),
              ),
              if (_isOwnProfile)
                IconButton(
                  icon: const Icon(Icons.menu_rounded, color: Colors.white),
                  onPressed: () => context.push('/settings'),
                ),
              const SizedBox(width: 4),
            ],
          ),
          // Profile header
          SliverToBoxAdapter(
            child: _buildProfileHeader(profile, userId),
          ),
          // Sticky tab bar
          SliverPersistentHeader(
            delegate: _StickyTabBarDelegate(tabBar),
            pinned: true,
          ),
        ],
        body: TabBarView(
          controller: _tabController,
          children: [
            VideoGridTab(userId: userId, tab: VideoTab.posted),
            VideoGridTab(userId: userId, tab: VideoTab.liked),
            if (_isOwnProfile)
              VideoGridTab(userId: userId, tab: VideoTab.bookmarked),
          ],
        ),
      ),
    );
  }

  // ── Main build ─────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    if (widget.userId == null) {
      // Own profile — derive userId from auth state.
      final ownAsync = ref.watch(ownProfileProvider);
      return ownAsync.when(
        loading: () => const Scaffold(
          backgroundColor: Colors.black,
          body: Center(child: CircularProgressIndicator()),
        ),
        error: (e, _) => Scaffold(
          backgroundColor: Colors.black,
          body: Center(
              child: Text(e.toString(),
                  style: const TextStyle(color: Colors.white))),
        ),
        data: (own) {
          if (own == null) {
            return const Scaffold(
              backgroundColor: Colors.black,
              body: Center(
                  child: Text('Not signed in',
                      style: TextStyle(color: Colors.white))),
            );
          }
          _resolvedUserId = own.userId;
          _isOwnProfile = true;
          return _buildScaffold(own, own.userId);
        },
      );
    }

    // Another user's profile.
    _resolvedUserId = widget.userId;
    _isOwnProfile = false;
    final profileAsync = ref.watch(profileProvider(widget.userId!));

    return profileAsync.when(
      loading: () => const Scaffold(
        backgroundColor: Colors.black,
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (e, _) => Scaffold(
        backgroundColor: Colors.black,
        appBar: AppBar(backgroundColor: Colors.black),
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(e.toString(),
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: Colors.white)),
              const SizedBox(height: 12),
              TextButton(
                onPressed: () => ref
                    .read(profileProvider(widget.userId!).notifier)
                    .refresh(),
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      ),
      data: (profile) => _buildScaffold(profile, widget.userId!),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Small private button widgets shared within this file
// ─────────────────────────────────────────────────────────────────────────────

class _OutlinedActionButton extends StatelessWidget {
  const _OutlinedActionButton({
    required this.label,
    required this.onTap,
  });

  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return OutlinedButton(
      onPressed: onTap,
      style: OutlinedButton.styleFrom(
        foregroundColor: Colors.white,
        side: const BorderSide(color: Colors.white24),
        padding: const EdgeInsets.symmetric(vertical: 9),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
      ),
      child: Text(label,
          style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13)),
    );
  }
}

class _GradientFollowButton extends StatelessWidget {
  const _GradientFollowButton({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xFFEE1D52), Color(0xFFFF006A)],
        ),
        borderRadius: BorderRadius.circular(6),
      ),
      child: ElevatedButton(
        onPressed: onTap,
        style: ElevatedButton.styleFrom(
          backgroundColor: Colors.transparent,
          shadowColor: Colors.transparent,
          padding: const EdgeInsets.symmetric(vertical: 9),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
        ),
        child: const Text('Follow',
            style: TextStyle(fontWeight: FontWeight.w700, fontSize: 13)),
      ),
    );
  }
}

class _OutlinedIconButton extends StatelessWidget {
  const _OutlinedIconButton({
    required this.icon,
    required this.onTap,
  });

  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return OutlinedButton(
      onPressed: onTap,
      style: OutlinedButton.styleFrom(
        foregroundColor: Colors.white,
        side: const BorderSide(color: Colors.white24),
        minimumSize: const Size(42, 42),
        padding: EdgeInsets.zero,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
      ),
      child: Icon(icon, size: 20),
    );
  }
}
