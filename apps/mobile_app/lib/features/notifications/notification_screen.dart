import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class NotificationScreen extends StatefulWidget {
  const NotificationScreen({super.key});

  @override
  State<NotificationScreen> createState() => _NotificationScreenState();
}

class _NotificationScreenState extends State<NotificationScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  final List<_NotifData> _all = [
    _NotifData(
      id: '1',
      type: _NotifType.like,
      username: 'john_doe',
      message: 'liked your video',
      time: DateTime.now().subtract(const Duration(minutes: 2)),
      isRead: false,
      actionRoute: '/video/video_001',
    ),
    _NotifData(
      id: '2',
      type: _NotifType.follow,
      username: 'sarah_k',
      message: 'started following you',
      time: DateTime.now().subtract(const Duration(minutes: 15)),
      isRead: false,
      actionRoute: '/profile/sarah_k',
    ),
    _NotifData(
      id: '3',
      type: _NotifType.comment,
      username: 'alex99',
      message: 'commented: "Fire content 🔥"',
      time: DateTime.now().subtract(const Duration(hours: 1)),
      isRead: false,
      actionRoute: '/comments/video_001',
    ),
    _NotifData(
      id: '4',
      type: _NotifType.like,
      username: 'mike_j',
      message: 'and 12 others liked your video',
      time: DateTime.now().subtract(const Duration(hours: 3)),
      isRead: true,
      actionRoute: '/video/video_002',
    ),
    _NotifData(
      id: '5',
      type: _NotifType.live,
      username: 'creator_xyz',
      message: 'just went LIVE — join now!',
      time: DateTime.now().subtract(const Duration(hours: 5)),
      isRead: true,
      actionRoute: '/live/stream_001',
    ),
    _NotifData(
      id: '6',
      type: _NotifType.gift,
      username: 'fan_user',
      message: 'sent you a Rose gift in your stream',
      time: DateTime.now().subtract(const Duration(days: 1)),
      isRead: true,
      actionRoute: '/wallet',
    ),
    _NotifData(
      id: '7',
      type: _NotifType.mention,
      username: 'techtalks',
      message: 'mentioned you in a comment',
      time: DateTime.now().subtract(const Duration(days: 1)),
      isRead: true,
      actionRoute: '/comments/video_003',
    ),
    _NotifData(
      id: '8',
      type: _NotifType.system,
      username: 'TikTok Clone',
      message: 'Your video reached 1K views! 🎉',
      time: DateTime.now().subtract(const Duration(days: 2)),
      isRead: true,
      actionRoute: '/video/video_001',
    ),
    _NotifData(
      id: '9',
      type: _NotifType.follow,
      username: 'creative_mind',
      message: 'started following you',
      time: DateTime.now().subtract(const Duration(days: 2)),
      isRead: true,
      actionRoute: '/profile/creative_mind',
    ),
    _NotifData(
      id: '10',
      type: _NotifType.comment,
      username: 'foodieemma',
      message: 'replied to your comment',
      time: DateTime.now().subtract(const Duration(days: 3)),
      isRead: true,
      actionRoute: '/comments/video_002',
    ),
  ];

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

  List<_NotifData> get _unread =>
      _all.where((n) => !n.isRead).toList();

  List<_NotifData> _filtered(String tab) {
    switch (tab) {
      case 'likes':
        return _all
            .where((n) => n.type == _NotifType.like)
            .toList();
      case 'comments':
        return _all
            .where((n) =>
                n.type == _NotifType.comment ||
                n.type == _NotifType.mention)
            .toList();
      default:
        return _all;
    }
  }

  void _markRead(String id) {
    setState(() {
      final index = _all.indexWhere((n) => n.id == id);
      if (index != -1) {
        _all[index] = _all[index].copyWith(isRead: true);
      }
    });
  }

  void _markAllRead() {
    setState(() {
      for (int i = 0; i < _all.length; i++) {
        _all[i] = _all[i].copyWith(isRead: true);
      }
    });
  }

  void _dismiss(String id) {
    setState(() {
      _all.removeWhere((n) => n.id == id);
    });
  }

  void _onTap(_NotifData notif) {
    _markRead(notif.id);
    if (notif.actionRoute.isNotEmpty) {
      context.push(notif.actionRoute);
    }
  }

  @override
  Widget build(BuildContext context) {
    final unreadCount = _unread.length;

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
          // IconButton(
          //   icon: const Icon(Icons.settings_outlined,
          //       color: Colors.white, size: 20),
          //   onPressed: () =>
          //       context.push('/settings/notification-preferences'),
          // ),
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
      body: TabBarView(
        controller: _tabController,
        children: [
          _NotifList(
            notifications: _filtered('all'),
            onTap: _onTap,
            onDismiss: _dismiss,
          ),
          _NotifList(
            notifications: _filtered('likes'),
            onTap: _onTap,
            onDismiss: _dismiss,
          ),
          _NotifList(
            notifications: _filtered('comments'),
            onTap: _onTap,
            onDismiss: _dismiss,
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Notification list
// ---------------------------------------------------------------------------

class _NotifList extends StatelessWidget {
  final List<_NotifData> notifications;
  final void Function(_NotifData) onTap;
  final void Function(String) onDismiss;

  const _NotifList({
    required this.notifications,
    required this.onTap,
    required this.onDismiss,
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
        return Dismissible(
          key: ValueKey(n.id),
          direction: DismissDirection.endToStart,
          background: Container(
            alignment: Alignment.centerRight,
            padding: const EdgeInsets.only(right: 20),
            color: const Color(0xFFEE1D52),
            child: const Icon(Icons.delete_outline,
                color: Colors.white, size: 24),
          ),
          onDismissed: (_) => onDismiss(n.id),
          child: _NotifTile(notif: n, onTap: () => onTap(n)),
        );
      },
    );
  }
}

// ---------------------------------------------------------------------------
// Notification tile
// ---------------------------------------------------------------------------

class _NotifTile extends StatelessWidget {
  final _NotifData notif;
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
            // Avatar with icon badge
            Stack(
              children: [
                CircleAvatar(
                  radius: 24,
                  backgroundColor: const Color(0xFF2A2A2A),
                  child: Text(
                    notif.username.isNotEmpty
                        ? notif.username[0].toUpperCase()
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
                      border: Border.all(
                          color: Colors.black, width: 1.5),
                    ),
                    child: Icon(notif.type.icon,
                        color: Colors.white, size: 11),
                  ),
                ),
              ],
            ),
            const SizedBox(width: 12),
            // Content
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  RichText(
                    text: TextSpan(
                      children: [
                        TextSpan(
                          text: '@${notif.username} ',
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
                    _timeAgo(notif.time),
                    style: const TextStyle(
                        color: Colors.white38, fontSize: 12),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            // Unread dot / action button
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
                if (notif.type == _NotifType.follow)
                  const SizedBox(height: 8),
                if (notif.type == _NotifType.follow)
                  _FollowBackButton(),
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
        padding: const EdgeInsets.symmetric(
            horizontal: 12, vertical: 6),
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

// ---------------------------------------------------------------------------
// Data model
// ---------------------------------------------------------------------------

enum _NotifType {
  like,
  follow,
  comment,
  mention,
  live,
  gift,
  system;

  IconData get icon {
    switch (this) {
      case _NotifType.like:
        return Icons.favorite;
      case _NotifType.follow:
        return Icons.person_add;
      case _NotifType.comment:
        return Icons.comment;
      case _NotifType.mention:
        return Icons.alternate_email;
      case _NotifType.live:
        return Icons.live_tv;
      case _NotifType.gift:
        return Icons.card_giftcard;
      case _NotifType.system:
        return Icons.notifications;
    }
  }

  Color get color {
    switch (this) {
      case _NotifType.like:
        return const Color(0xFFEE1D52);
      case _NotifType.follow:
        return const Color(0xFF69C9D0);
      case _NotifType.comment:
        return Colors.orange;
      case _NotifType.mention:
        return Colors.blue;
      case _NotifType.live:
        return Colors.red;
      case _NotifType.gift:
        return Colors.purple;
      case _NotifType.system:
        return Colors.green;
    }
  }
}

class _NotifData {
  final String id;
  final _NotifType type;
  final String username;
  final String message;
  final DateTime time;
  final bool isRead;
  final String actionRoute;

  const _NotifData({
    required this.id,
    required this.type,
    required this.username,
    required this.message,
    required this.time,
    required this.isRead,
    required this.actionRoute,
  });

  _NotifData copyWith({bool? isRead}) {
    return _NotifData(
      id: id,
      type: type,
      username: username,
      message: message,
      time: time,
      isRead: isRead ?? this.isRead,
      actionRoute: actionRoute,
    );
  }
}