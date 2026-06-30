import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// Formats an integer count to a compact human-readable string.
/// 12 500     → "12.5K"
/// 1 200 000  → "1.2M"
/// 999        → "999"
String formatCount(int count) {
  if (count >= 1000000) {
    final m = count / 1000000;
    return '${_stripTrailingZero(m.toStringAsFixed(1))}M';
  }
  if (count >= 1000) {
    final k = count / 1000;
    return '${_stripTrailingZero(k.toStringAsFixed(1))}K';
  }
  return count.toString();
}

String _stripTrailingZero(String s) {
  if (s.endsWith('.0')) return s.substring(0, s.length - 2);
  return s;
}

// ─────────────────────────────────────────────────────────────────────────────

/// Displays follower / following / like counts in a horizontal row.
/// Each section animates its count up on first appearance.
/// Tapping followers or following navigates to the respective list screen.
class ProfileStatsRow extends StatefulWidget {
  const ProfileStatsRow({
    super.key,
    required this.userId,
    required this.followerCount,
    required this.followingCount,
    required this.likeCount,
    this.animate = true,
  });

  final String userId;
  final int followerCount;
  final int followingCount;
  final int likeCount;

  /// Whether to run the count-up animation on first build.
  final bool animate;

  @override
  State<ProfileStatsRow> createState() => _ProfileStatsRowState();
}

class _ProfileStatsRowState extends State<ProfileStatsRow>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  late final Animation<double> _progress;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 900),
    );
    _progress = CurvedAnimation(
      parent: _controller,
      curve: Curves.easeOut,
    );

    if (widget.animate) {
      // Small delay so the profile header has time to settle.
      Future.delayed(const Duration(milliseconds: 150), () {
        if (mounted) _controller.forward();
      });
    } else {
      _controller.value = 1.0;
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _progress,
      builder: (context, _) {
        final t = _progress.value;
        final animatedFollowers =
            (widget.followerCount * t).round();
        final animatedFollowing =
            (widget.followingCount * t).round();
        final animatedLikes = (widget.likeCount * t).round();

        return IntrinsicHeight(
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              _StatCell(
                label: 'Followers',
                value: formatCount(animatedFollowers),
                onTap: () => context.push('/followers/${widget.userId}'),
              ),
              
              _Divider(),
              _StatCell(
                label: 'Following',
                value: formatCount(animatedFollowing),
                onTap: () => context.push('/following/${widget.userId}'),
              ),
              
              _Divider(),
              _StatCell(
                label: 'Likes',
                value: formatCount(animatedLikes),
                onTap: null, // likes count is non-navigable
              ),
            ],
          ),
        );
      },
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────

class _StatCell extends StatelessWidget {
  const _StatCell({
    required this.label,
    required this.value,
    required this.onTap,
  });

  final String label;
  final String value;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    final content = Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 4),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            value,
            style: theme.textTheme.titleLarge?.copyWith(
              fontWeight: FontWeight.w700,
              fontSize: 18,
              letterSpacing: -0.3,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurface.withValues(alpha: 0.55),
              fontSize: 12,
            ),
          ),
        ],
      ),
    );

    if (onTap == null) return content;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: content,
    );
  }
}

class _Divider extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      width: 1,
      margin: const EdgeInsets.symmetric(vertical: 8),
      color: Theme.of(context).dividerColor.withValues(alpha: 0.35),
    );
  }
}
