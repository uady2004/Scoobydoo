import 'package:tiktok_clone/features/notifications/domain/entities/notification_entity.dart';

class NotificationModel extends NotificationEntity {
  const NotificationModel({
    required super.id,
    required super.type,
    required super.actorId,
    required super.actorUsername,
    super.actorAvatarUrl,
    required super.message,
    super.targetId,
    super.targetType,
    super.thumbnailUrl,
    required super.isRead,
    required super.createdAt,
  });

  factory NotificationModel.fromJson(Map<String, dynamic> j) => NotificationModel(
    id: j['id'] as String,
    type: _parseType(j['type'] as String? ?? 'system'),
    actorId: j['actor_id'] as String? ?? '',
    actorUsername: j['actor_username'] as String? ?? '',
    actorAvatarUrl: j['actor_avatar_url'] as String?,
    message: j['message'] as String? ?? '',
    targetId: j['target_id'] as String?,
    targetType: j['target_type'] as String?,
    thumbnailUrl: j['thumbnail_url'] as String?,
    isRead: j['is_read'] as bool? ?? false,
    createdAt: DateTime.parse(j['created_at'] as String),
  );

  Map<String, dynamic> toJson() => {
    'id': id,
    'type': type.name,
    'actor_id': actorId,
    'actor_username': actorUsername,
    'actor_avatar_url': actorAvatarUrl,
    'message': message,
    'target_id': targetId,
    'target_type': targetType,
    'thumbnail_url': thumbnailUrl,
    'is_read': isRead,
    'created_at': createdAt.toIso8601String(),
  };

  static NotificationType _parseType(String t) => switch (t) {
    'like' => NotificationType.like,
    'comment' => NotificationType.comment,
    'follow' => NotificationType.follow,
    'mention' => NotificationType.mention,
    'duet' => NotificationType.duet,
    'stitch' => NotificationType.stitch,
    'live' => NotificationType.live,
    _ => NotificationType.system,
  };
}
