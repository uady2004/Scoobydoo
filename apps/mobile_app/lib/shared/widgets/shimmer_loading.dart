import 'package:flutter/material.dart';
import 'package:shimmer/shimmer.dart';

import '../../core/theme/app_colors.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Base shimmer wrapper
// ─────────────────────────────────────────────────────────────────────────────

/// Wraps [child] in a shimmer sweep animation using the app's dark palette.
///
/// The base color is [AppColors.surfaceVariant] and the highlight sweeps to a
/// lighter grey — visible against the black background without being harsh.
class ShimmerLoading extends StatelessWidget {
  const ShimmerLoading({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Shimmer.fromColors(
      baseColor: AppColors.surfaceVariant,
      highlightColor: const Color(0xFF3E3E3E),
      period: const Duration(milliseconds: 1200),
      child: child,
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Feed shimmer — 3 full-width video-card placeholders
// ─────────────────────────────────────────────────────────────────────────────

/// Three stacked 200px full-width grey boxes mimicking the video feed.
class FeedShimmer extends StatelessWidget {
  const FeedShimmer({super.key});

  @override
  Widget build(BuildContext context) {
    return ShimmerLoading(
      child: Column(
        children: List.generate(3, (index) => const _FeedCardPlaceholder()),
      ),
    );
  }
}

class _FeedCardPlaceholder extends StatelessWidget {
  const _FeedCardPlaceholder();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      height: 200,
      margin: const EdgeInsets.only(bottom: 8),
      decoration: BoxDecoration(
        color: AppColors.surfaceVariant,
        borderRadius: BorderRadius.circular(4),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Profile shimmer — avatar circle + 3 line-bar rows
// ─────────────────────────────────────────────────────────────────────────────

/// A profile-header placeholder: a 72px circle followed by three text bars
/// of decreasing width.
class ProfileShimmer extends StatelessWidget {
  const ProfileShimmer({super.key});

  @override
  Widget build(BuildContext context) {
    return ShimmerLoading(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            // Avatar circle
            Container(
              width: 72,
              height: 72,
              decoration: const BoxDecoration(
                color: AppColors.surfaceVariant,
                shape: BoxShape.circle,
              ),
            ),
            const SizedBox(height: 16),
            // Line bars
            const _LineBar(width: 160, height: 16),
            const SizedBox(height: 8),
            const _LineBar(width: 120, height: 12),
            const SizedBox(height: 8),
            const _LineBar(width: 200, height: 12),
          ],
        ),
      ),
    );
  }
}

class _LineBar extends StatelessWidget {
  const _LineBar({required this.width, required this.height});

  final double width;
  final double height;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: AppColors.surfaceVariant,
        borderRadius: BorderRadius.circular(4),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Grid shimmer — 3 × 3 grey squares
// ─────────────────────────────────────────────────────────────────────────────

/// A 3-column grid of grey square placeholders, matching the profile video
/// grid layout.
class GridShimmer extends StatelessWidget {
  const GridShimmer({super.key, this.crossAxisCount = 3, this.itemCount = 9});

  final int crossAxisCount;
  final int itemCount;

  @override
  Widget build(BuildContext context) {
    return ShimmerLoading(
      child: GridView.builder(
        shrinkWrap: true,
        physics: const NeverScrollableScrollPhysics(),
        gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: crossAxisCount,
          crossAxisSpacing: 2,
          mainAxisSpacing: 2,
          childAspectRatio: 0.75,
        ),
        itemCount: itemCount,
        itemBuilder: (_, __) => const _GridCell(),
      ),
    );
  }
}

class _GridCell extends StatelessWidget {
  const _GridCell();

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppColors.surfaceVariant,
    );
  }
}
