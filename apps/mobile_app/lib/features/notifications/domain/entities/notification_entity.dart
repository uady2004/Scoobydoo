enum NotificationType { like, comment, follow, mention, duet, stitch, live, system }

class NotificationEntity {
  const NotificationEntity({
    required this.id,
    required this.type,
    required this.actorId,
    required this.actorUsername,
    required this.actorAvatarUrl,
    required this.message,
    this.targetId,
    this.targetType,
    this.thumbnailUrl,
    required this.isRead,
    required this.createdAt,
  });

  final String id;
  final NotificationType type;
  final String actorId;
  final String actorUsername;
  final String? actorAvatarUrl;
  final String message;
  final String? targetId;
  final String? targetType;
  final String? thumbnailUrl;
  final bool isRead;
  final DateTime createdAt;

  NotificationEntity copyWith({bool? isRead}) => NotificationEntity(
    id: id, type: type, actorId: actorId, actorUsername: actorUsername,
    actorAvatarUrl: actorAvatarUrl, message: message, targetId: targetId,
    targetType: targetType, thumbnailUrl: thumbnailUrl,
    isRead: isRead ?? this.isRead, createdAt: createdAt,
  );
}
