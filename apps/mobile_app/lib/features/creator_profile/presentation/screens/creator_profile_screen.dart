import 'dart:io';

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

// ─────────────────────────────────────────────────────────────────────────────
// Creator analytics teaser card
// ─────────────────────────────────────────────────────────────────────────────

class _AnalyticsTeaserCard extends StatelessWidget {
  const _AnalyticsTeaserCard({
    required this.weeklyViews,
    required this.weeklyFollowers,
    required this.weeklyRevenue,
  });

  final int weeklyViews;
  final int weeklyFollowers;
  final double weeklyRevenue;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => Navigator.pushNamed(context, '/creator-analytics'),
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 16),
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          gradient: const LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [Color(0xFF1A1A2E), Color(0xFF16213E)],
          ),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
              color: Colors.white.withValues(alpha: 0.08), width: 1),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.bar_chart_rounded,
                    color: Color(0xFF20D5EC), size: 18),
                const SizedBox(width: 8),
                const Text(
                  'This week',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 0.2,
                  ),
                ),
                const Spacer(),
                Text(
                  'Full analytics',
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.4),
                    fontSize: 12,
                  ),
                ),
                const SizedBox(width: 4),
                Icon(Icons.chevron_right_rounded,
                    color: Colors.white.withValues(alpha: 0.4),
                    size: 16),
              ],
            ),
            const SizedBox(height: 14),
            Row(
              children: [
                Expanded(
                  child: _AnalyticsStat(
                    label: 'Views',
                    value: formatCount(weeklyViews),
                    icon: Icons.play_circle_outline_rounded,
                    color: const Color(0xFF20D5EC),
                  ),
                ),
                Expanded(
                  child: _AnalyticsStat(
                    label: 'Followers',
                    value: '+${formatCount(weeklyFollowers)}',
                    icon: Icons.person_add_outlined,
                    color: const Color(0xFF69C9D0),
                  ),
                ),
                Expanded(
                  child: _AnalyticsStat(
                    label: 'Revenue',
                    value:
                        '\$${weeklyRevenue.toStringAsFixed(weeklyRevenue < 10 ? 2 : 0)}',
                    icon: Icons.attach_money_rounded,
                    color: const Color(0xFFFFD700),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _AnalyticsStat extends StatelessWidget {
  const _AnalyticsStat({
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
  });

  final String label;
  final String value;
  final IconData icon;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Icon(icon, color: color, size: 20),
        const SizedBox(height: 4),
        Text(
          value,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 15,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: 2),
        Text(
          label,
          style: TextStyle(
            color: Colors.white.withValues(alpha: 0.45),
            fontSize: 11,
          ),
        ),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// LIVE button row
// ─────────────────────────────────────────────────────────────────────────────

class _LiveRow extends StatelessWidget {
  const _LiveRow();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          // Go LIVE button
          Expanded(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: const LinearGradient(
                  colors: [Color(0xFFEE1D52), Color(0xFFFF006A)],
                ),
                borderRadius: BorderRadius.circular(8),
              ),
              child: ElevatedButton.icon(
                onPressed: () =>
                    Navigator.pushNamed(context, '/livestream/create'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: Colors.transparent,
                  shadowColor: Colors.transparent,
                  padding: const EdgeInsets.symmetric(vertical: 11),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
                icon: const Icon(Icons.circle,
                    color: Colors.white, size: 10),
                label: const Text(
                  'Go LIVE',
                  style: TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w800,
                    fontSize: 14,
                    letterSpacing: 0.5,
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(width: 10),
          // Schedule LIVE
          OutlinedButton.icon(
            onPressed: () =>
                Navigator.pushNamed(context, '/livestream/schedule'),
            style: OutlinedButton.styleFrom(
              foregroundColor: Colors.white,
              side: const BorderSide(color: Colors.white24),
              padding:
                  const EdgeInsets.symmetric(vertical: 11, horizontal: 16),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8)),
            ),
            icon: const Icon(Icons.calendar_today_outlined, size: 16),
            label: const Text('Schedule',
                style: TextStyle(
                    fontSize: 13, fontWeight: FontWeight.w600)),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Shop section — horizontal scroll of product thumbnail cards
// ─────────────────────────────────────────────────────────────────────────────

class _ShopProduct {
  const _ShopProduct({
    required this.id,
    required this.imageUrl,
    required this.name,
    required this.price,
  });

  final String id;
  final String imageUrl;
  final String name;
  final double price;
}

class _ShopSection extends StatelessWidget {
  const _ShopSection({required this.creatorId});

  final String creatorId;

  // In a real app this would come from a provider; here we show the
  // structural shell with placeholder-safe rendering.
  static const List<_ShopProduct> _demo = [];

  @override
  Widget build(BuildContext context) {
    if (_demo.isEmpty) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
          child: Row(
            children: [
              const Icon(Icons.storefront_outlined,
                  color: Colors.white70, size: 18),
              const SizedBox(width: 8),
              const Text(
                'Shop',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const Spacer(),
              GestureDetector(
                onTap: () => Navigator.pushNamed(
                    context, '/shop/$creatorId'),
                child: Text(
                  'View all',
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.4),
                    fontSize: 12,
                  ),
                ),
              ),
            ],
          ),
        ),
        SizedBox(
          height: 168,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            itemCount: _demo.length,
            separatorBuilder: (_, __) => const SizedBox(width: 10),
            itemBuilder: (context, index) {
              final product = _demo[index];
              return _ProductCard(product: product);
            },
          ),
        ),
      ],
    );
  }
}

class _ProductCard extends StatelessWidget {
  const _ProductCard({required this.product});

  final _ShopProduct product;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () =>
          Navigator.pushNamed(context, '/product/${product.id}'),
      child: SizedBox(
        width: 120,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: CachedNetworkImage(
                imageUrl: product.imageUrl,
                width: 120,
                height: 120,
                fit: BoxFit.cover,
                placeholder: (_, __) =>
                    Container(color: Colors.grey.shade900),
                errorWidget: (_, __, ___) =>
                    Container(color: Colors.grey.shade900),
              ),
            ),
            const SizedBox(height: 6),
            Text(
              product.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                  color: Colors.white, fontSize: 12),
            ),
            Text(
              '\$${product.price.toStringAsFixed(2)}',
              style: const TextStyle(
                color: Color(0xFF20D5EC),
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Sliver persistent header delegate (copied locally to avoid cross-file dep)
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
// Creator profile screen
// ─────────────────────────────────────────────────────────────────────────────

/// Extends the standard profile layout with creator-specific sections:
/// - LIVE row (Go LIVE + Schedule)
/// - Analytics teaser card (this week: views / followers / revenue)
/// - Shop section (horizontal product scroll)
///
/// Pass [userId] null to show the signed-in creator's own profile.
class CreatorProfileScreen extends ConsumerStatefulWidget {
  const CreatorProfileScreen({super.key, this.userId});

  final String? userId;

  @override
  ConsumerState<CreatorProfileScreen> createState() =>
      _CreatorProfileScreenState();
}

class _CreatorProfileScreenState
    extends ConsumerState<CreatorProfileScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  bool _bioExpanded = false;
  bool _isOwnProfile = false;

  @override
  void initState() {
    super.initState();
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
        'start', '--user', '0',
        '-a', 'android.intent.action.VIEW',
        '-d', url,
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
    ref.read(followStateProvider(userId).notifier).state = !isFollowing;

    final result = await ref.read(followUserUseCaseProvider).call(
          FollowUserParams(userId: userId, isFollowing: isFollowing),
        );

    result.fold(
      (failure) {
        ref.read(followStateProvider(userId).notifier).state =
            isFollowing;
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(failure.message)),
          );
        }
      },
      (_) {
        ref.read(profileProvider(userId)).whenData((profile) {
          final delta = isFollowing ? -1 : 1;
          ref.read(profileProvider(userId).notifier).applyLocalUpdate(
                profile.copyWith(
                    followerCount: profile.followerCount + delta),
              );
        });
      },
    );
  }

  // ── Avatar ─────────────────────────────────────────────────────────────────

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

  // ── Profile header (own creator) ───────────────────────────────────────────

  Widget _buildOwnCreatorActions(ProfileEntity profile) {
    return Row(
      children: [
        Expanded(
          child: OutlinedButton(
            onPressed: () => Navigator.pushNamed(
              context,
              '/edit-profile',
              arguments: profile,
            ),
            style: OutlinedButton.styleFrom(
              foregroundColor: Colors.white,
              side: const BorderSide(color: Colors.white24),
              padding: const EdgeInsets.symmetric(vertical: 9),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(6)),
            ),
            child: const Text('Edit profile',
                style: TextStyle(
                    fontWeight: FontWeight.w600, fontSize: 13)),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: OutlinedButton(
            onPressed: () =>
                Navigator.pushNamed(context, '/creator-tools'),
            style: OutlinedButton.styleFrom(
              foregroundColor: Colors.white,
              side: const BorderSide(color: Colors.white24),
              padding: const EdgeInsets.symmetric(vertical: 9),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(6)),
            ),
            child: const Text('Creator tools',
                style: TextStyle(
                    fontWeight: FontWeight.w600, fontSize: 13)),
          ),
        ),
      ],
    );
  }

  Widget _buildOtherCreatorActions(String userId) {
    final isFollowing = ref.watch(followStateProvider(userId));
    return Row(
      children: [
        Expanded(
          flex: 3,
          child: isFollowing
              ? OutlinedButton(
                  onPressed: () => _toggleFollow(userId),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: Colors.white,
                    side: const BorderSide(color: Colors.white24),
                    padding: const EdgeInsets.symmetric(vertical: 9),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(6)),
                  ),
                  child: const Text('Following',
                      style: TextStyle(
                          fontWeight: FontWeight.w600, fontSize: 13)),
                )
              : DecoratedBox(
                  decoration: BoxDecoration(
                    gradient: const LinearGradient(
                      colors: [Color(0xFFEE1D52), Color(0xFFFF006A)],
                    ),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: ElevatedButton(
                    onPressed: () => _toggleFollow(userId),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: Colors.transparent,
                      shadowColor: Colors.transparent,
                      padding: const EdgeInsets.symmetric(vertical: 9),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(6)),
                    ),
                    child: const Text('Follow',
                        style: TextStyle(
                            fontWeight: FontWeight.w700, fontSize: 13)),
                  ),
                ),
        ),
        const SizedBox(width: 8),
        Expanded(
          flex: 2,
          child: OutlinedButton(
            onPressed: () =>
                Navigator.pushNamed(context, '/messages/$userId'),
            style: OutlinedButton.styleFrom(
              foregroundColor: Colors.white,
              side: const BorderSide(color: Colors.white24),
              padding: const EdgeInsets.symmetric(vertical: 9),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(6)),
            ),
            child: const Text('Message',
                style: TextStyle(
                    fontWeight: FontWeight.w600, fontSize: 13)),
          ),
        ),
        const SizedBox(width: 8),
        OutlinedButton(
          onPressed: () => _showMoreSheet(userId),
          style: OutlinedButton.styleFrom(
            foregroundColor: Colors.white,
            side: const BorderSide(color: Colors.white24),
            minimumSize: const Size(42, 42),
            padding: EdgeInsets.zero,
            shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(6)),
          ),
          child: const Icon(Icons.more_horiz_rounded, size: 20),
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
              leading: const Icon(Icons.block_rounded),
              title: const Text('Block'),
              onTap: () => Navigator.pop(context),
            ),
            ListTile(
              leading: const Icon(Icons.flag_outlined),
              title: const Text('Report'),
              onTap: () {
                Navigator.pop(context);
                Navigator.pushNamed(context, '/report/user/$userId');
              },
            ),
          ],
        ),
      ),
    );
  }

  // ── Profile header section ─────────────────────────────────────────────────

  Widget _buildProfileHeader(ProfileEntity profile, String userId) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          _buildAvatar(profile),
          const SizedBox(height: 10),
          Text(
            '@${profile.username}',
            style: const TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w700,
              letterSpacing: -0.2,
            ),
          ),
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
          if (profile.bio != null && profile.bio!.isNotEmpty)
            GestureDetector(
              onTap: () =>
                  setState(() => _bioExpanded = !_bioExpanded),
              child: Text(
                profile.bio!,
                textAlign: TextAlign.center,
                maxLines: _bioExpanded ? null : 2,
                overflow: _bioExpanded
                    ? TextOverflow.visible
                    : TextOverflow.ellipsis,
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.85),
                  fontSize: 13,
                  height: 1.45,
                ),
              ),
            ),
          if (profile.website != null &&
              profile.website!.isNotEmpty) ...[
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
            _buildOwnCreatorActions(profile)
          else
            _buildOtherCreatorActions(userId),
          const SizedBox(height: 4),
        ],
      ),
    );
  }

  // ── Creator extras (LIVE + analytics + shop) ───────────────────────────────

  /// Builds the creator-specific sliver content between the profile header
  /// and the tab bar.
  Widget _buildCreatorExtras(ProfileEntity profile, String userId) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 12),

        // LIVE row — only shown on own profile.
        if (_isOwnProfile) ...[
          const _LiveRow(),
          const SizedBox(height: 12),
        ],

        // Analytics teaser — own profile only.
        if (_isOwnProfile) ...[
          _AnalyticsTeaserCard(
            weeklyViews: 148200,
            weeklyFollowers: 3400,
            weeklyRevenue: 212.50,
          ),
          const SizedBox(height: 12),
        ],

        // Shop section — visible to all viewers of a creator profile.
        _ShopSection(creatorId: userId),
        const SizedBox(height: 8),
      ],
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
          const Tab(
              icon: Icon(Icons.bookmark_border_rounded, size: 22)),
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
                    icon: const Icon(
                        Icons.arrow_back_ios_new_rounded,
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
                icon: const Icon(Icons.share_outlined,
                    color: Colors.white),
                onPressed: () => _shareProfile(profile),
              ),
              if (_isOwnProfile)
                IconButton(
                  icon: const Icon(Icons.menu_rounded,
                      color: Colors.white),
                  onPressed: () =>
                      Navigator.pushNamed(context, '/settings'),
                ),
              const SizedBox(width: 4),
            ],
          ),

          // Profile header
          SliverToBoxAdapter(
            child: _buildProfileHeader(profile, userId),
          ),

          // Creator extras (LIVE, analytics, shop)
          SliverToBoxAdapter(
            child: _buildCreatorExtras(profile, userId),
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
                style: const TextStyle(color: Colors.white)),
          ),
        ),
        data: (own) {
          if (own == null) {
            return const Scaffold(
              backgroundColor: Colors.black,
              body: Center(
                child: Text('Not signed in',
                    style: TextStyle(color: Colors.white)),
              ),
            );
          }
          _isOwnProfile = true;
          return _buildScaffold(own, own.userId);
        },
      );
    }

    _isOwnProfile = false;
    final profileAsync =
        ref.watch(profileProvider(widget.userId!));

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
