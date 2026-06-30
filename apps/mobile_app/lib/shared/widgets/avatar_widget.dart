import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

/// A circular avatar backed by [CachedNetworkImage].
///
/// Falls back to initials on a grey circle when [imageUrl] is null or fails.
/// Optionally shows:
///   - a blue verified tick (bottom-right, 16px)
///   - an online green dot (top-right, 10px)
///
/// ```dart
/// AvatarWidget(
///   imageUrl: user.avatarUrl,
///   initials: user.initials,
///   radius: 24,
///   isVerified: user.isVerified,
///   isOnline: user.isOnline,
/// )
/// ```
class AvatarWidget extends StatelessWidget {
  const AvatarWidget({
    super.key,
    this.imageUrl,
    required this.initials,
    this.radius = 20,
    this.isVerified = false,
    this.isOnline = false,
    this.borderColor,
    this.borderWidth = 0,
    this.onTap,
  });

  final String? imageUrl;
  final String initials;
  final double radius;
  final bool isVerified;
  final bool isOnline;
  final Color? borderColor;
  final double borderWidth;
  final VoidCallback? onTap;

  double get _diameter => radius * 2;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: _diameter + (borderWidth > 0 ? borderWidth * 2 : 0) + 12,
        height: _diameter + (borderWidth > 0 ? borderWidth * 2 : 0) + 12,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            // ── Avatar ──────────────────────────────────────────────────────
            Positioned(
              left: 0,
              top: 0,
              child: _buildCircle(),
            ),

            // ── Verified tick ────────────────────────────────────────────────
            if (isVerified)
              const Positioned(
                right: 0,
                bottom: 0,
                child: _VerifiedBadge(size: 16),
              ),

            // ── Online dot ───────────────────────────────────────────────────
            if (isOnline)
              const Positioned(
                right: 0,
                top: 0,
                child: _OnlineDot(size: 10),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildCircle() {
    final circle = Container(
      width: _diameter,
      height: _diameter,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: borderWidth > 0 && borderColor != null
            ? Border.all(color: borderColor!, width: borderWidth)
            : null,
      ),
      child: ClipOval(
        child: imageUrl != null && imageUrl!.isNotEmpty
            ? CachedNetworkImage(
                imageUrl: imageUrl!,
                fit: BoxFit.cover,
                width: _diameter,
                height: _diameter,
                placeholder: (context, url) => _Placeholder(initials: initials),
                errorWidget: (context, url, error) =>
                    _Placeholder(initials: initials),
              )
            : _Placeholder(initials: initials),
      ),
    );
    return circle;
  }
}

// ── Sub-widgets ──────────────────────────────────────────────────────────────

class _Placeholder extends StatelessWidget {
  const _Placeholder({required this.initials});
  final String initials;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: const Color(0xFF3D3D3D),
      child: Center(
        child: Text(
          initials.isNotEmpty
              ? initials.substring(0, initials.length.clamp(0, 2)).toUpperCase()
              : '?',
          style: const TextStyle(
            color: Colors.white,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}

class _VerifiedBadge extends StatelessWidget {
  const _VerifiedBadge({required this.size});
  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: const BoxDecoration(
        color: AppColors.verified,
        shape: BoxShape.circle,
      ),
      child: Icon(
        Icons.check,
        size: size * 0.65,
        color: Colors.white,
      ),
    );
  }
}

class _OnlineDot extends StatelessWidget {
  const _OnlineDot({required this.size});
  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: AppColors.online,
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.background, width: 1.5),
      ),
    );
  }
}
