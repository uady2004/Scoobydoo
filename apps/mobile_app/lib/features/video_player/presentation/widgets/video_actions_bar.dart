import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../home_feed/domain/entities/feed_item_entity.dart';
import '../../../home_feed/presentation/providers/feed_provider.dart';

String _formatCount(int count) {
  if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
  if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
  return count.toString();
}

class _ActionButton extends StatelessWidget {
  const _ActionButton({
    required this.icon,
    required this.label,
    this.onTap,
    this.semanticLabel,
  });

  final Widget icon;
  final String label;
  final VoidCallback? onTap;
  final String? semanticLabel;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: semanticLabel ?? label,
      button: true,
      child: GestureDetector(
        onTap: onTap,
        behavior: HitTestBehavior.opaque,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            icon,
            const SizedBox(height: 4),
            Text(
              label,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 12,
                fontWeight: FontWeight.w600,
                shadows: [Shadow(color: Colors.black54, blurRadius: 6)],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _RotatingDisc extends StatefulWidget {
  const _RotatingDisc({required this.imageUrl, required this.isPlaying});

  final String imageUrl;
  final bool isPlaying;

  @override
  State<_RotatingDisc> createState() => _RotatingDiscState();
}

class _RotatingDiscState extends State<_RotatingDisc>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 6),
    );
    if (widget.isPlaying) _ctrl.repeat();
  }

  @override
  void didUpdateWidget(_RotatingDisc old) {
    super.didUpdateWidget(old);
    if (widget.isPlaying && !_ctrl.isAnimating) {
      _ctrl.repeat();
    } else if (!widget.isPlaying && _ctrl.isAnimating) {
      _ctrl.stop();
    }
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return RotationTransition(
      turns: _ctrl,
      child: Container(
        width: 44,
        height: 44,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          border: Border.all(color: Colors.white38, width: 2),
          color: Colors.black54,
        ),
        child: ClipOval(
          child: CachedNetworkImage(
            imageUrl: widget.imageUrl,
            fit: BoxFit.cover,
            errorWidget: (_, __, ___) => const Icon(
              Icons.music_note,
              color: Colors.white,
              size: 22,
            ),
          ),
        ),
      ),
    );
  }
}

class VideoActionsBar extends ConsumerWidget {
  const VideoActionsBar({
    super.key,
    required this.item,
    required this.isPlaying,
    required this.feedType,
    this.onCommentTap,
    this.onShareTap,
    this.onSoundTap,
    this.onUsernameTap,       // ← ADDED
  });

  final FeedItemEntity item;
  final bool isPlaying;
  final String feedType;
  final VoidCallback? onCommentTap;
  final VoidCallback? onShareTap;
  final VoidCallback? onSoundTap;
  final VoidCallback? onUsernameTap;  // ← ADDED

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Positioned(
      right: 8,
      bottom: 80,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // ── Creator avatar + follow button ───────────────────────────────
          _CreatorAvatar(
            item: item,
            onFollowTap: () => _toggleFollow(ref),
            onAvatarTap: onUsernameTap,   // ← ADDED
          ),
          const SizedBox(height: 20),

          // ── Like ─────────────────────────────────────────────────────────
          _ActionButton(
            semanticLabel:
                '${_formatCount(item.likeCount)} likes. ${item.isLiked ? 'Unlike' : 'Like'} this video.',
            icon: AnimatedSwitcher(
              duration: const Duration(milliseconds: 200),
              transitionBuilder: (child, anim) =>
                  ScaleTransition(scale: anim, child: child),
              child: Icon(
                item.isLiked ? Icons.favorite : Icons.favorite_border,
                key: ValueKey(item.isLiked),
                color: item.isLiked ? Colors.red : Colors.white,
                size: 35,
                shadows: const [Shadow(color: Colors.black54, blurRadius: 6)],
              ),
            ),
            label: _formatCount(item.likeCount),
            onTap: () => _toggleLike(ref),
          ),
          const SizedBox(height: 16),

          // ── Comments ─────────────────────────────────────────────────────
          _ActionButton(
            semanticLabel: '${_formatCount(item.commentCount)} comments',
            icon: const Icon(
              Icons.chat_bubble_outline,
              color: Colors.white,
              size: 35,
              shadows: [Shadow(color: Colors.black54, blurRadius: 6)],
            ),
            label: _formatCount(item.commentCount),
            onTap: onCommentTap,
          ),
          const SizedBox(height: 16),

          // ── Bookmark ─────────────────────────────────────────────────────
          _ActionButton(
            semanticLabel:
                '${item.isBookmarked ? 'Remove bookmark' : 'Bookmark'} — ${_formatCount(item.bookmarkCount)} saves',
            icon: AnimatedSwitcher(
              duration: const Duration(milliseconds: 200),
              transitionBuilder: (child, anim) =>
                  ScaleTransition(scale: anim, child: child),
              child: Icon(
                item.isBookmarked ? Icons.bookmark : Icons.bookmark_border,
                key: ValueKey(item.isBookmarked),
                color: item.isBookmarked
                    ? const Color(0xFFFFE55C)
                    : Colors.white,
                size: 35,
                shadows: const [Shadow(color: Colors.black54, blurRadius: 6)],
              ),
            ),
            label: _formatCount(item.bookmarkCount),
            onTap: () => _toggleBookmark(ref),
          ),
          const SizedBox(height: 16),

          // ── Share ────────────────────────────────────────────────────────
          _ActionButton(
            semanticLabel: 'Share — ${_formatCount(item.shareCount)} shares',
            icon: const Icon(
              Icons.reply,
              color: Colors.white,
              size: 35,
              shadows: [Shadow(color: Colors.black54, blurRadius: 6)],
            ),
            label: _formatCount(item.shareCount),
            onTap: onShareTap,
          ),
          const SizedBox(height: 16),

          // ── Sound disc ───────────────────────────────────────────────────
          Semantics(
            label: 'Open sound: ${item.soundTitle}',
            button: true,
            child: GestureDetector(
              onTap: onSoundTap,
              child: _RotatingDisc(
                imageUrl: item.thumbnailUrl,
                isPlaying: isPlaying,
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _toggleLike(WidgetRef ref) {
    if (feedType == 'following') {
      ref.read(followingFeedProvider.notifier).toggleLike(item.videoId);
    } else {
      ref.read(forYouFeedProvider.notifier).toggleLike(item.videoId);
    }
  }

  void _toggleBookmark(WidgetRef ref) {
    if (feedType == 'following') {
      ref.read(followingFeedProvider.notifier).toggleBookmark(item.videoId);
    } else {
      ref.read(forYouFeedProvider.notifier).toggleBookmark(item.videoId);
    }
  }

  void _toggleFollow(WidgetRef ref) {
    if (feedType == 'following') {
      ref.read(followingFeedProvider.notifier).toggleFollow(item.creatorId);
    } else {
      ref.read(forYouFeedProvider.notifier).toggleFollow(item.creatorId);
    }
  }
}

class _CreatorAvatar extends StatelessWidget {
  const _CreatorAvatar({
    required this.item,
    required this.onFollowTap,
    this.onAvatarTap,         // ← ADDED
  });

  final FeedItemEntity item;
  final VoidCallback onFollowTap;
  final VoidCallback? onAvatarTap;  // ← ADDED

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 52,
      child: Stack(
        clipBehavior: Clip.none,
        alignment: Alignment.center,
        children: [
          // Avatar ring — tappable to open profile
          GestureDetector(
            onTap: onAvatarTap,   // ← ADDED
            child: Container(
              width: 52,
              height: 52,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: Colors.white, width: 1.5),
              ),
              child: ClipOval(
                child: CachedNetworkImage(
                  imageUrl: item.creatorAvatarUrl,
                  fit: BoxFit.cover,
                  errorWidget: (_, __, ___) => const CircleAvatar(
                    backgroundColor: Color(0xFF333333),
                    child: Icon(Icons.person, color: Colors.white),
                  ),
                ),
              ),
            ),
          ),

          // + / check follow badge
          Positioned(
            bottom: -10,
            child: GestureDetector(
              onTap: onFollowTap,
              behavior: HitTestBehavior.opaque,
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 200),
                width: 22,
                height: 22,
                decoration: BoxDecoration(
                  color: item.isFollowing
                      ? const Color(0xFF555555)
                      : const Color(0xFFFF0050),
                  shape: BoxShape.circle,
                  border: Border.all(color: Colors.black, width: 1.5),
                ),
                child: Icon(
                  item.isFollowing ? Icons.check : Icons.add,
                  color: Colors.white,
                  size: 14,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}