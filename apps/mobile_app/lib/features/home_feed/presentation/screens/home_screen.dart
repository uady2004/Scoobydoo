import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../for_you_feed/presentation/screens/for_you_screen.dart';
import '../../../following_feed/presentation/screens/following_screen.dart';

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return const DefaultTabController(
      length: 2,
      initialIndex: 1, // Start on "For You"
      child: Scaffold(
        extendBodyBehindAppBar: true,
        backgroundColor: Colors.black,
        appBar: _HomeAppBar(),
        body: Stack(
          children: [
            TabBarView(
              // Disable horizontal swipe — users swipe vertically inside PageViews.
              physics: NeverScrollableScrollPhysics(),
              children: [
                FollowingScreen(),
                ForYouScreen(),
              ],
            ),
            // ── Live notification banner ───────────────────────────────
            _LiveNotificationBanner(),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// App bar
// ─────────────────────────────────────────────────────────────────────────────

class _HomeAppBar extends StatelessWidget implements PreferredSizeWidget {
  const _HomeAppBar();

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight + 2);

  @override
  Widget build(BuildContext context) {
    return AppBar(
      backgroundColor: Colors.transparent,
      elevation: 0,
      // Camera icon — left
      leading: Semantics(
        label: 'Open camera',
        child: IconButton(
          icon: const Icon(Icons.videocam_outlined, color: Colors.white, size: 26),
          onPressed: () => context.push('/upload'),
          tooltip: 'Camera',
        ),
      ),
      // Tab bar — centre
      title: const _FeedTabBar(),
      centerTitle: true,
      // Notification bell — right
      actions: [
        Semantics(
          label: 'Notifications',
          child: IconButton(
            icon: const Icon(
              Icons.notifications_outlined,
              color: Colors.white,
              size: 26,
            ),
            onPressed: () => context.push('/notifications'),
            tooltip: 'Notifications',
          ),
        ),
        const SizedBox(width: 4),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Tab bar — "Following" | "For You"
// ─────────────────────────────────────────────────────────────────────────────

class _FeedTabBar extends StatelessWidget {
  const _FeedTabBar();

  @override
  Widget build(BuildContext context) {
    return const TabBar(
      isScrollable: true,
      tabAlignment: TabAlignment.center,
      dividerColor: Colors.transparent,
      indicator: UnderlineTabIndicator(
        borderSide: BorderSide(color: Colors.white, width: 2),
        insets: EdgeInsets.symmetric(horizontal: 8),
      ),
      indicatorSize: TabBarIndicatorSize.label,
      labelColor: Colors.white,
      unselectedLabelColor: Colors.white54,
      labelStyle: TextStyle(
        fontWeight: FontWeight.w700,
        fontSize: 16,
        letterSpacing: 0.1,
      ),
      unselectedLabelStyle: TextStyle(
        fontWeight: FontWeight.w400,
        fontSize: 15,
      ),
      tabs: [
        Tab(text: 'Following'),
        Tab(text: 'For You'),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Live notification banner
// ─────────────────────────────────────────────────────────────────────────────

class _LiveNotificationBanner extends StatefulWidget {
  const _LiveNotificationBanner();

  @override
  State<_LiveNotificationBanner> createState() =>
      _LiveNotificationBannerState();
}

class _LiveNotificationBannerState extends State<_LiveNotificationBanner>
    with SingleTickerProviderStateMixin {
  bool _visible = true;
  late AnimationController _ctrl;
  late Animation<Offset> _slide;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 400),
    );
    _slide = Tween<Offset>(
      begin: const Offset(0, -1),
      end: Offset.zero,
    ).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeOut));

    // Show after 2 seconds
    Future.delayed(const Duration(seconds: 2), () {
      if (mounted) _ctrl.forward();
    });

    // Auto hide after 7 seconds
    Future.delayed(const Duration(seconds: 7), () {
      if (mounted) {
        _ctrl.reverse().then((_) {
          if (mounted) setState(() => _visible = false);
        });
      }
    });
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_visible) return const SizedBox.shrink();

    return Positioned(
      top: MediaQuery.of(context).padding.top + kToolbarHeight + 8,
      left: 12,
      right: 12,
      child: SlideTransition(
        position: _slide,
        child: GestureDetector(
          onTap: () => context.push('/live/stream_001'),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            decoration: BoxDecoration(
              color: const Color(0xFF1A1A1A).withValues(alpha: 0.95),
              borderRadius: BorderRadius.circular(12),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.4),
                  blurRadius: 12,
                ),
              ],
            ),
            child: Row(
              children: [
                // Avatar with LIVE badge
                Stack(
                  children: [
                    const CircleAvatar(
                      radius: 22,
                      backgroundColor: Color(0xFF333333),
                      child: Icon(Icons.person,
                          color: Colors.white, size: 24),
                    ),
                    Positioned(
                      bottom: 0,
                      left: 0,
                      right: 0,
                      child: Container(
                        margin:
                            const EdgeInsets.symmetric(horizontal: 2),
                        padding:
                            const EdgeInsets.symmetric(vertical: 1),
                        decoration: BoxDecoration(
                          color: const Color(0xFFFF2D55),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: const Text(
                          'LIVE',
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 8,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(width: 10),
                // Name + subtitle
                const Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Alex Johnson',
                        style: TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.bold,
                          fontSize: 14,
                        ),
                      ),
                      Text(
                        'Is live now, come watch!',
                        style: TextStyle(
                            color: Colors.white54, fontSize: 12),
                      ),
                    ],
                  ),
                ),
                // Close button
                GestureDetector(
                  onTap: () {
                    _ctrl.reverse().then((_) {
                      if (mounted) setState(() => _visible = false);
                    });
                  },
                  child: const Icon(Icons.close,
                      color: Colors.white38, size: 18),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}