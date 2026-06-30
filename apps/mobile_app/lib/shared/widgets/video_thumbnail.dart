import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_text_styles.dart';
import 'shimmer_loading.dart';

/// A video thumbnail with a gradient fade, play icon, and duration label.
///
/// Fills its parent container. Wrap in a [SizedBox] or [AspectRatio] to
/// control dimensions.
///
/// ```dart
/// SizedBox(
///   width: 120,
///   height: 160,
///   child: VideoThumbnail(
///     thumbnailUrl: video.thumbnailUrl,
///     duration: video.formattedDuration,
///   ),
/// )
/// ```
class VideoThumbnail extends StatelessWidget {
  const VideoThumbnail({
    super.key,
    required this.thumbnailUrl,
    this.duration,
    this.showPlayIcon = true,
    this.borderRadius = 4,
    this.onTap,
    this.isLive = false,
  });

  final String thumbnailUrl;

  /// Display string e.g. `"1:23"`. Hidden when null.
  final String? duration;
  final bool showPlayIcon;
  final double borderRadius;
  final VoidCallback? onTap;

  /// When true, shows a LIVE badge instead of the duration.
  final bool isLive;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(borderRadius),
        child: Stack(
          fit: StackFit.expand,
          children: [
            // ── Thumbnail image ──────────────────────────────────────────────
            CachedNetworkImage(
              imageUrl: thumbnailUrl,
              fit: BoxFit.cover,
              placeholder: (context, url) => const ShimmerLoading(
                child: ColoredBox(color: AppColors.surfaceVariant),
              ),
              errorWidget: (context, url, error) => const ColoredBox(
                color: AppColors.surfaceVariant,
                child: Icon(
                  Icons.broken_image_outlined,
                  color: AppColors.textTertiary,
                  size: 32,
                ),
              ),
            ),

            // ── Bottom gradient fade ─────────────────────────────────────────
            const Positioned(
              left: 0,
              right: 0,
              bottom: 0,
              height: 80,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: AppColors.gradientDark,
                ),
              ),
            ),

            // ── Play icon (centred) ──────────────────────────────────────────
            if (showPlayIcon && !isLive)
              const Center(
                child: Icon(
                  Icons.play_circle_fill_rounded,
                  color: Colors.white54,
                  size: 36,
                ),
              ),

            // ── Duration / LIVE label (bottom-left) ──────────────────────────
            Positioned(
              left: 6,
              bottom: 6,
              child: isLive ? const _LiveBadge() : _DurationLabel(duration),
            ),
          ],
        ),
      ),
    );
  }
}

// ── Sub-widgets ──────────────────────────────────────────────────────────────

class _DurationLabel extends StatelessWidget {
  const _DurationLabel(this.duration);
  final String? duration;

  @override
  Widget build(BuildContext context) {
    if (duration == null || duration!.isEmpty) return const SizedBox.shrink();
    return Text(
      duration!,
      style: AppTextStyles.captionSemibold.copyWith(
        shadows: [
          const Shadow(
            color: Colors.black54,
            blurRadius: 4,
          ),
        ],
      ),
    );
  }
}

class _LiveBadge extends StatelessWidget {
  const _LiveBadge();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: AppColors.live,
        borderRadius: BorderRadius.circular(4),
      ),
      child: const Text(
        'LIVE',
        style: TextStyle(
          color: Colors.white,
          fontSize: 10,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.5,
        ),
      ),
    );
  }
}
