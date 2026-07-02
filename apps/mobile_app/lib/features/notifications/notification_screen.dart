import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/notifications/domain/entities/notification_entity.dart';
import 'package:tiktok_clone/features/notifications/presentation/providers/notification_provider.dart';

extension _NotifTypeUI on NotificationType {
  IconData get icon {
    switch (this) {
      case NotificationType.like:
        return Icons.favorite;
      case NotificationType.comment:
        return Icons.comment;
      case NotificationType.follow:
        return Icons.person_add;
      case NotificationType.mention:
        return Icons.alternate_email;
      case NotificationType.duet:
        return Icons.compare_arrows;
      case NotificationType.stitch:
        return Icons.merge_type;
      case NotificationType.live:
        return Icons.live_tv;
      case NotificationType.system:
        return Icons.notifications;
    }
  }

  Color get color {
    switch (this) {
      case NotificationType.like:
        return const Color(0xFFEE1D52);
      case NotificationType.comment:
        return Colors.orange;
      case NotificationType.follow:
        return const Color(0xFF69C9D0);
      case NotificationType.mention:
        return Colors.blue;
      case NotificationType.duet:
        return Colors.purple;
      case NotificationType.stitch:
        return Colors.green;
      case NotificationType.live:
        return Colors.red;
      case NotificationType.system:
        return Colors.teal;
    }
  }
}

class NotificationScreen extends ConsumerStatefulWidget {
  const NotificationScreen({super.key});

  @override
  ConsumerState<NotificationScreen> createState() =>
      _NotificationScreenState();
}

class _NotificationScreenState extends ConsumerState<NotificationScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

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

  List<NotificationEntity> _filtered(
      List<NotificationEntity> all, String tab) {
    switch (tab) {
      case 'likes':
        return all.where((n) => n.type == NotificationType.like).toList();
      case 'comments':
        return all
            .where((n) =>
                n.type == NotificationType.comment ||
                n.type == NotificationType.mention)
            .toList();
      default:
        return all;
    }
  }

  void _markRead(String id) =>
      ref.read(notificationsProvider.notifier).markRead(id);

  void _markAllRead() =>
      ref.read(notificationsProvider.notifier).markAllRead();

  void _onTap(NotificationEntity notif) {
    _markRead(notif.id);
    final route = _routeFor(notif);
    if (route != null && mounted) context.push(route);
  }

  String? _routeFor(NotificationEntity notif) {
    switch (notif.type) {
      case NotificationType.follow:
        return '/profile/${notif.actorUsername}';
      case NotificationType.like:
      case NotificationType.comment:
      case NotificationType.mention:
      case NotificationType.duet:
      case NotificationType.stitch:
        if (notif.targetId != null) return '/comments/${notif.targetId}';
        return null;
      default:
        return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(notificationsProvider);
    final all = state.notifications;
    final unreadCount = all.where((n) => !n.isRead).length;

    Widget body;
    if (state.isLoading && all.isEmpty) {
      body = const Center(
        child: CircularProgressIndicator(
          valueColor: AlwaysStoppedAnimation(Color(0xFFEE1D52)),
          strokeWidth: 2,
        ),
      );
    } else if (state.error != null && all.isEmpty) {
      body = Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline,
                color: Colors.white38, size: 48),
            const SizedBox(height: 12),
            Text(
              state.error!,
              style: const TextStyle(color: Colors.white38, fontSize: 14),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            TextButton(
              onPressed: () =>
                  ref.read(notificationsProvider.notifier).load(),
              child: const Text('Try again',
                  style: TextStyle(color: Color(0xFFEE1D52))),
            ),
          ],
        ),
      );
    } else {
      body = TabBarView(
        controller: _tabController,
        children: [
          _NotifList(
            notifications: _filtered(all, 'all'),
            onTap: _onTap,
          ),
          _NotifList(
            notifications: _filtered(all, 'likes'),
            onTap: _onTap,
          ),
          _NotifList(
            notifications: _filtered(all, 'comments'),
            onTap: _onTap,
          ),
        ],
      );
    }

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new,
              color: Colors.white, size: 18),
          onPressed: () => context.pop(),
        ),
        title: Row(
          children: [
            const Text(
              'Notifications',
              style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.w700,
              ),
            ),
            if (unreadCount > 0) ...[
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(
                    horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: const Color(0xFFEE1D52),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Text(
                  '$unreadCount',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 11,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
            ],
          ],
        ),
        actions: [
          if (unreadCount > 0)
            TextButton(
              onPressed: _markAllRead,
              child: const Text(
                'Mark all read',
                style: TextStyle(
                    color: Color(0xFFEE1D52), fontSize: 13),
              ),
            ),
        ],
        bottom: TabBar(
          controller: _tabController,
          indicatorColor: const Color(0xFFEE1D52),
          labelColor: Colors.white,
          unselectedLabelColor: Colors.white38,
          tabs: const [
            Tab(text: 'All'),
            Tab(text: 'Likes'),
            Tab(text: 'Comments'),
          ],
        ),
      ),
      body: body,
    );
  }
}

// ---------------------------------------------------------------------------
// Notification list
// ---------------------------------------------------------------------------

class _NotifList extends StatelessWidget {
  final List<NotificationEntity> notifications;
  final void Function(NotificationEntity) onTap;

  const _NotifList({
    required this.notifications,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    if (notifications.isEmpty) {
      return const Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('🔔', style: TextStyle(fontSize: 48)),
            SizedBox(height: 12),
            Text(
              'No notifications yet',
              style: TextStyle(color: Colors.white54, fontSize: 15),
            ),
          ],
        ),
      );
    }

    return ListView.separated(
      itemCount: notifications.length,
      separatorBuilder: (_, __) =>
          const Divider(color: Color(0xFF1A1A1A), height: 1),
      itemBuilder: (context, index) {
        final n = notifications[index];
        return _NotifTile(notif: n, onTap: () => onTap(n));
      },
    );
  }
}

// ---------------------------------------------------------------------------
// Notification tile
// ---------------------------------------------------------------------------

class _NotifTile extends StatelessWidget {
  final NotificationEntity notif;
  final VoidCallback onTap;

  const _NotifTile({required this.notif, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      splashColor: Colors.white10,
      child: Container(
        color: notif.isRead
            ? Colors.transparent
            : const Color(0xFF1A1A1A),
        padding: const EdgeInsets.symmetric(
            horizontal: 16, vertical: 12),
        child: Row(
          children: [
            Stack(
              children: [
                notif.actorAvatarUrl != null &&
                        notif.actorAvatarUrl!.isNotEmpty
                    ? CircleAvatar(
                        radius: 24,
                        backgroundImage:
                            NetworkImage(notif.actorAvatarUrl!),
                        backgroundColor: const Color(0xFF2A2A2A),
                      )
                    : CircleAvatar(
                        radius: 24,
                        backgroundColor: const Color(0xFF2A2A2A),
                        child: Text(
                          notif.actorUsername.isNotEmpty
                              ? notif.actorUsername[0].toUpperCase()
                              : '?',
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 18,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                Positioned(
                  right: 0,
                  bottom: 0,
                  child: Container(
                    width: 20,
                    height: 20,
                    decoration: BoxDecoration(
                      color: notif.type.color,
                      shape: BoxShape.circle,
                      border:
                          Border.all(color: Colors.black, width: 1.5),
                    ),
                    child: Icon(notif.type.icon,
                        color: Colors.white, size: 11),
                  ),
                ),
              ],
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  RichText(
                    text: TextSpan(
                      children: [
                        TextSpan(
                          text: '@${notif.actorUsername} ',
                          style: const TextStyle(
                            color: Colors.white,
                            fontWeight: FontWeight.w700,
                            fontSize: 14,
                          ),
                        ),
                        TextSpan(
                          text: notif.message,
                          style: TextStyle(
                            color: notif.isRead
                                ? Colors.white60
                                : Colors.white,
                            fontSize: 14,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    _timeAgo(notif.createdAt),
                    style: const TextStyle(
                        color: Colors.white38, fontSize: 12),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            Column(
              children: [
                if (!notif.isRead)
                  Container(
                    width: 8,
                    height: 8,
                    decoration: const BoxDecoration(
                      color: Color(0xFFEE1D52),
                      shape: BoxShape.circle,
                    ),
                  ),
                if (notif.type == NotificationType.follow) ...[
                  const SizedBox(height: 8),
                  _FollowBackButton(),
                ],
              ],
            ),
          ],
        ),
      ),
    );
  }

  String _timeAgo(DateTime dt) {
    final diff = DateTime.now().difference(dt);
    if (diff.inMinutes < 1) return 'Just now';
    if (diff.inHours < 1) return '${diff.inMinutes}m ago';
    if (diff.inDays < 1) return '${diff.inHours}h ago';
    if (diff.inDays < 7) return '${diff.inDays}d ago';
    return '${dt.day}/${dt.month}/${dt.year}';
  }
}

// ---------------------------------------------------------------------------
// Follow back button
// ---------------------------------------------------------------------------

class _FollowBackButton extends StatefulWidget {
  @override
  State<_FollowBackButton> createState() => _FollowBackButtonState();
}

class _FollowBackButtonState extends State<_FollowBackButton> {
  bool _following = false;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => setState(() => _following = !_following),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        padding:
            const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: _following
              ? Colors.transparent
              : const Color(0xFFEE1D52),
          border: Border.all(
            color: _following
                ? Colors.white38
                : const Color(0xFFEE1D52),
          ),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Text(
          _following ? 'Following' : 'Follow back',
          style: const TextStyle(
            color: Colors.white,
            fontSize: 12,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}
