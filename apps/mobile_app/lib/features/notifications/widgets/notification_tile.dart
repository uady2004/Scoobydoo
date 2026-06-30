import 'package:flutter/material.dart';

import '../../../shared/models/notification_model.dart';

/// NotificationTile renders a single notification row.
/// Unread notifications have a subtle left accent and a tinted background.
class NotificationTile extends StatelessWidget {
  final NotificationModel notification;
  final VoidCallback onTap;

  const NotificationTile({
    super.key,
    required this.notification,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final unread = !notification.isRead;

    return InkWell(
      onTap: onTap,
      child: Container(
        decoration: BoxDecoration(
          color: unread
              ? const Color(0xFF1A1A1A)
              : Colors.transparent,
          border: Border(
            left: BorderSide(
              color: unread ? const Color(0xFFFE2C55) : Colors.transparent,
              width: 3,
            ),
            bottom: BorderSide(color: Colors.grey[900]!, width: 0.5),
          ),
        ),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _NotificationAvatar(notification: notification),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _NotificationBody(notification: notification),
                  const SizedBox(height: 4),
                  _TimestampLabel(createdAt: notification.createdAt),
                ],
              ),
            ),
            if (notification.imageUrl != null &&
                notification.imageUrl!.isNotEmpty)
              _ThumbnailImage(url: notification.imageUrl!),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Avatar: actor photo or a type-based icon fallback.
// ---------------------------------------------------------------------------

class _NotificationAvatar extends StatelessWidget {
  final NotificationModel notification;

  const _NotificationAvatar({required this.notification});

  @override
  Widget build(BuildContext context) {
    final avatarUrl = notification.actorAvatar;
    final iconData = _iconForType(notification.type);
    final iconColor = _colorForType(notification.type);

    return Stack(
      clipBehavior: Clip.none,
      children: [
        CircleAvatar(
          radius: 22,
          backgroundColor: Colors.grey[800],
          backgroundImage:
              (avatarUrl != null && avatarUrl.isNotEmpty)
                  ? NetworkImage(avatarUrl)
                  : null,
          child: (avatarUrl == null || avatarUrl.isEmpty)
              ? Icon(Icons.person, color: Colors.grey[500], size: 22)
              : null,
        ),
        Positioned(
          bottom: -2,
          right: -2,
          child: Container(
            width: 20,
            height: 20,
            decoration: BoxDecoration(
              color: iconColor,
              shape: BoxShape.circle,
              border: Border.all(color: Colors.black, width: 1.5),
            ),
            child: Icon(iconData, color: Colors.white, size: 11),
          ),
        ),
      ],
    );
  }

  IconData _iconForType(NotificationType type) {
    switch (type) {
      case NotificationType.like:
        return Icons.favorite;
      case NotificationType.comment:
        return Icons.comment;
      case NotificationType.follow:
        return Icons.person_add;
      case NotificationType.mention:
        return Icons.alternate_email;
      case NotificationType.gift:
        return Icons.card_giftcard;
      case NotificationType.orderCreated:
      case NotificationType.orderShipped:
        return Icons.shopping_bag;
      case NotificationType.livestream:
        return Icons.live_tv;
      case NotificationType.system:
      case NotificationType.emailVerification:
      case NotificationType.passwordReset:
      case NotificationType.weeklyDigest:
        return Icons.notifications;
      case NotificationType.unknown:
        return Icons.circle_notifications;
    }
  }

  Color _colorForType(NotificationType type) {
    switch (type) {
      case NotificationType.like:
        return const Color(0xFFFE2C55);
      case NotificationType.comment:
        return const Color(0xFF25F4EE);
      case NotificationType.follow:
        return const Color(0xFF6C63FF);
      case NotificationType.mention:
        return const Color(0xFFFF9500);
      case NotificationType.gift:
        return const Color(0xFFFFD700);
      case NotificationType.orderCreated:
      case NotificationType.orderShipped:
        return const Color(0xFF34C759);
      case NotificationType.livestream:
        return const Color(0xFFFE2C55);
      default:
        return Colors.grey;
    }
  }
}

// ---------------------------------------------------------------------------
// Notification body: bold actor name inline with action text.
// ---------------------------------------------------------------------------

class _NotificationBody extends StatelessWidget {
  final NotificationModel notification;

  const _NotificationBody({required this.notification});

  @override
  Widget build(BuildContext context) {
    final actorName = notification.actorName;

    return RichText(
      text: TextSpan(
        style: const TextStyle(
          color: Colors.white,
          fontSize: 14,
          height: 1.4,
        ),
        children: [
          if (actorName != null && actorName.isNotEmpty)
            TextSpan(
              text: '$actorName ',
              style: const TextStyle(fontWeight: FontWeight.w700),
            ),
          TextSpan(text: notification.body),
        ],
      ),
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
    );
  }
}

// ---------------------------------------------------------------------------
// Relative timestamp label.
// ---------------------------------------------------------------------------

class _TimestampLabel extends StatelessWidget {
  final DateTime createdAt;

  const _TimestampLabel({required this.createdAt});

  @override
  Widget build(BuildContext context) {
    return Text(
      _relativeTime(createdAt),
      style: TextStyle(color: Colors.grey[600], fontSize: 12),
    );
  }

  static String _relativeTime(DateTime dt) {
    final diff = DateTime.now().difference(dt);
    if (diff.inSeconds < 60) return 'just now';
    if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
    if (diff.inHours < 24) return '${diff.inHours}h ago';
    if (diff.inDays < 7) return '${diff.inDays}d ago';
    if (diff.inDays < 30) return '${(diff.inDays / 7).floor()}w ago';
    if (diff.inDays < 365) return '${(diff.inDays / 30).floor()}mo ago';
    return '${(diff.inDays / 365).floor()}y ago';
  }
}

// ---------------------------------------------------------------------------
// Optional video thumbnail on the right side.
// ---------------------------------------------------------------------------

class _ThumbnailImage extends StatelessWidget {
  final String url;

  const _ThumbnailImage({required this.url});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: 12),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(4),
        child: Image.network(
          url,
          width: 48,
          height: 48,
          fit: BoxFit.cover,
          errorBuilder: (_, __, ___) => Container(
            width: 48,
            height: 48,
            color: Colors.grey[800],
            child: const Icon(Icons.broken_image, color: Colors.grey, size: 20),
          ),
        ),
      ),
    );
  }
}
