import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:timeago/timeago.dart' as timeago;
import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';

/// Renders a single comment row.
///
/// Displays the author avatar (CachedNetworkImage when [CommentEntity.avatarUrl]
/// is non-empty, initials fallback otherwise), username, relative timestamp,
/// comment body, an optimistic like button, and a "View N replies" link when
/// [CommentEntity.replyCount] is greater than zero.
class CommentTile extends StatelessWidget {
  const CommentTile({
    super.key,
    required this.comment,
    this.onLike,
    this.onViewReplies,
  });

  final CommentEntity comment;

  /// Called after the user taps the like button. Fire-and-forget; the tile
  /// updates its local count immediately without waiting for this callback.
  final VoidCallback? onLike;

  /// Called when the user taps "View N replies".
  final VoidCallback? onViewReplies;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _Avatar(username: comment.username, avatarUrl: comment.avatarUrl),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      '@${comment.username}',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 13,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      timeago.format(comment.createdAt),
                      style: TextStyle(
                        color: Colors.grey[500],
                        fontSize: 12,
                      ),
                    ),
                    const Spacer(),
                    _LikeButton(
                      initialCount: comment.likeCount,
                      initialLiked: comment.isLiked,
                      onLike: onLike,
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  comment.content,
                  style: const TextStyle(
                    color: Colors.white70,
                    fontSize: 14,
                    height: 1.4,
                  ),
                  maxLines: 10,
                  overflow: TextOverflow.ellipsis,
                ),
                if (comment.replyCount > 0)
                  TextButton(
                    onPressed: onViewReplies,
                    style: TextButton.styleFrom(
                      padding: EdgeInsets.zero,
                      minimumSize: Size.zero,
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      foregroundColor: const Color(0xFF20D5C4),
                    ),
                    child: Text(
                      'View ${comment.replyCount} '
                      '${comment.replyCount == 1 ? 'reply' : 'replies'} →',
                      style: const TextStyle(fontSize: 12),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Avatar
// ---------------------------------------------------------------------------

class _Avatar extends StatelessWidget {
  const _Avatar({required this.username, required this.avatarUrl});

  final String username;
  final String avatarUrl;

  @override
  Widget build(BuildContext context) {
    final hasUrl = avatarUrl.isNotEmpty;
    return CircleAvatar(
      radius: 18,
      backgroundColor: Colors.grey[800],
      backgroundImage: hasUrl
          ? CachedNetworkImageProvider(avatarUrl)
          : null,
      child: hasUrl
          ? null
          : Text(
              _initials(username),
              style: const TextStyle(
                color: Colors.white,
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
    );
  }

  /// Returns up to two uppercase characters from [username].
  static String _initials(String username) {
    final trimmed = username.replaceAll('@', '').trim();
    if (trimmed.isEmpty) return '?';
    final parts = trimmed.split(RegExp(r'[\s._-]+'));
    if (parts.length >= 2) {
      return '${parts[0][0]}${parts[1][0]}'.toUpperCase();
    }
    return trimmed.substring(0, trimmed.length.clamp(1, 2)).toUpperCase();
  }
}

// ---------------------------------------------------------------------------
// Like button — owns its own state for optimistic updates
// ---------------------------------------------------------------------------

class _LikeButton extends StatefulWidget {
  const _LikeButton({
    required this.initialCount,
    required this.initialLiked,
    this.onLike,
  });

  final int initialCount;
  final bool initialLiked;

  /// Optional callback fired after each toggle. The count is already updated
  /// locally before this is called.
  final VoidCallback? onLike;

  @override
  State<_LikeButton> createState() => _LikeButtonState();
}

class _LikeButtonState extends State<_LikeButton> {
  late bool _liked;
  late int _count;

  @override
  void initState() {
    super.initState();
    _liked = widget.initialLiked;
    _count = widget.initialCount;
  }

  void _toggle() {
    setState(() {
      _liked = !_liked;
      _count += _liked ? 1 : -1;
    });
    widget.onLike?.call();
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: _toggle,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              _liked ? Icons.favorite : Icons.favorite_border,
              size: 16,
              color: _liked ? Colors.red : Colors.grey[500],
            ),
            const SizedBox(width: 4),
            Text(
              _formatCount(_count),
              style: TextStyle(
                color: Colors.grey[500],
                fontSize: 12,
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Formats large counts: 1 200 → "1.2K", 1 200 000 → "1.2M".
  static String _formatCount(int n) {
    if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
    if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
    return '$n';
  }
}
