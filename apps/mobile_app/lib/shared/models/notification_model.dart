// Notification domain models mirroring the Go backend JSON contracts.

enum NotificationType {
  like,
  comment,
  follow,
  mention,
  gift,
  orderCreated,
  orderShipped,
  livestream,
  system,
  emailVerification,
  passwordReset,
  weeklyDigest,
  unknown;

  static NotificationType fromString(String? raw) {
    switch (raw) {
      case 'like':
        return NotificationType.like;
      case 'comment':
        return NotificationType.comment;
      case 'follow':
        return NotificationType.follow;
      case 'mention':
        return NotificationType.mention;
      case 'gift':
        return NotificationType.gift;
      case 'order_created':
        return NotificationType.orderCreated;
      case 'order_shipped':
        return NotificationType.orderShipped;
      case 'livestream':
        return NotificationType.livestream;
      case 'system':
        return NotificationType.system;
      case 'email_verification':
        return NotificationType.emailVerification;
      case 'password_reset':
        return NotificationType.passwordReset;
      case 'weekly_digest':
        return NotificationType.weeklyDigest;
      default:
        return NotificationType.unknown;
    }
  }
}

class NotificationModel {
  final String id;
  final String userId;
  final String? actorId;
  final String? actorName;
  final String? actorAvatar;
  final NotificationType type;
  final String title;
  final String body;
  final String? imageUrl;
  final String? deepLink;
  final Map<String, dynamic>? metadata;
  final String? groupKey;
  final int groupCount;
  final bool isRead;
  final DateTime? readAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  const NotificationModel({
    required this.id,
    required this.userId,
    this.actorId,
    this.actorName,
    this.actorAvatar,
    required this.type,
    required this.title,
    required this.body,
    this.imageUrl,
    this.deepLink,
    this.metadata,
    this.groupKey,
    this.groupCount = 1,
    required this.isRead,
    this.readAt,
    required this.createdAt,
    required this.updatedAt,
  });

  factory NotificationModel.fromJson(Map<String, dynamic> json) {
    return NotificationModel(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      actorId: json['actor_id'] as String?,
      actorName: json['actor_name'] as String?,
      actorAvatar: json['actor_avatar'] as String?,
      type: NotificationType.fromString(json['type'] as String?),
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      imageUrl: json['image_url'] as String?,
      deepLink: json['deep_link'] as String?,
      metadata: json['metadata'] as Map<String, dynamic>?,
      groupKey: json['group_key'] as String?,
      groupCount: json['group_count'] as int? ?? 1,
      isRead: json['is_read'] as bool? ?? false,
      readAt: json['read_at'] != null
          ? DateTime.parse(json['read_at'] as String)
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }

  NotificationModel copyWith({bool? isRead, DateTime? readAt}) {
    return NotificationModel(
      id: id,
      userId: userId,
      actorId: actorId,
      actorName: actorName,
      actorAvatar: actorAvatar,
      type: type,
      title: title,
      body: body,
      imageUrl: imageUrl,
      deepLink: deepLink,
      metadata: metadata,
      groupKey: groupKey,
      groupCount: groupCount,
      isRead: isRead ?? this.isRead,
      readAt: readAt ?? this.readAt,
      createdAt: createdAt,
      updatedAt: updatedAt,
    );
  }
}

class NotificationsResponse {
  final List<NotificationModel> notifications;
  final int total;
  final int unreadCount;
  final int limit;
  final int offset;

  const NotificationsResponse({
    required this.notifications,
    required this.total,
    required this.unreadCount,
    required this.limit,
    required this.offset,
  });

  factory NotificationsResponse.fromJson(Map<String, dynamic> json) {
    final rawList = json['notifications'] as List<dynamic>? ?? [];
    return NotificationsResponse(
      notifications: rawList
          .map((e) => NotificationModel.fromJson(e as Map<String, dynamic>))
          .toList(),
      total: json['total'] as int? ?? 0,
      unreadCount: json['unread_count'] as int? ?? 0,
      limit: json['limit'] as int? ?? 20,
      offset: json['offset'] as int? ?? 0,
    );
  }
}

class NotificationPreference {
  final bool pushEnabled;
  final bool emailEnabled;
  final bool smsEnabled;
  final bool inAppEnabled;
  final bool likesEnabled;
  final bool commentsEnabled;
  final bool followsEnabled;
  final bool mentionsEnabled;
  final bool giftsEnabled;
  final bool ordersEnabled;
  final bool livestreamEnabled;
  final bool systemEnabled;
  final bool quietHoursEnabled;
  final String? quietStart;
  final String? quietEnd;
  final String? timezone;
  final bool digestEnabled;
  final String? digestFrequency;

  const NotificationPreference({
    this.pushEnabled = true,
    this.emailEnabled = true,
    this.smsEnabled = false,
    this.inAppEnabled = true,
    this.likesEnabled = true,
    this.commentsEnabled = true,
    this.followsEnabled = true,
    this.mentionsEnabled = true,
    this.giftsEnabled = true,
    this.ordersEnabled = true,
    this.livestreamEnabled = true,
    this.systemEnabled = true,
    this.quietHoursEnabled = false,
    this.quietStart,
    this.quietEnd,
    this.timezone,
    this.digestEnabled = false,
    this.digestFrequency,
  });

  factory NotificationPreference.fromJson(Map<String, dynamic> json) {
    return NotificationPreference(
      pushEnabled: json['push_enabled'] as bool? ?? true,
      emailEnabled: json['email_enabled'] as bool? ?? true,
      smsEnabled: json['sms_enabled'] as bool? ?? false,
      inAppEnabled: json['in_app_enabled'] as bool? ?? true,
      likesEnabled: json['likes_enabled'] as bool? ?? true,
      commentsEnabled: json['comments_enabled'] as bool? ?? true,
      followsEnabled: json['follows_enabled'] as bool? ?? true,
      mentionsEnabled: json['mentions_enabled'] as bool? ?? true,
      giftsEnabled: json['gifts_enabled'] as bool? ?? true,
      ordersEnabled: json['orders_enabled'] as bool? ?? true,
      livestreamEnabled: json['livestream_enabled'] as bool? ?? true,
      systemEnabled: json['system_enabled'] as bool? ?? true,
      quietHoursEnabled: json['quiet_hours_enabled'] as bool? ?? false,
      quietStart: json['quiet_start'] as String?,
      quietEnd: json['quiet_end'] as String?,
      timezone: json['timezone'] as String?,
      digestEnabled: json['digest_enabled'] as bool? ?? false,
      digestFrequency: json['digest_frequency'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'push_enabled': pushEnabled,
      'email_enabled': emailEnabled,
      'sms_enabled': smsEnabled,
      'in_app_enabled': inAppEnabled,
      'likes_enabled': likesEnabled,
      'comments_enabled': commentsEnabled,
      'follows_enabled': followsEnabled,
      'mentions_enabled': mentionsEnabled,
      'gifts_enabled': giftsEnabled,
      'orders_enabled': ordersEnabled,
      'livestream_enabled': livestreamEnabled,
      'system_enabled': systemEnabled,
      'quiet_hours_enabled': quietHoursEnabled,
      if (quietStart != null) 'quiet_start': quietStart,
      if (quietEnd != null) 'quiet_end': quietEnd,
      if (timezone != null) 'timezone': timezone,
      'digest_enabled': digestEnabled,
      if (digestFrequency != null) 'digest_frequency': digestFrequency,
    };
  }

  NotificationPreference copyWith({
    bool? pushEnabled,
    bool? emailEnabled,
    bool? smsEnabled,
    bool? inAppEnabled,
    bool? likesEnabled,
    bool? commentsEnabled,
    bool? followsEnabled,
    bool? mentionsEnabled,
    bool? giftsEnabled,
    bool? ordersEnabled,
    bool? livestreamEnabled,
    bool? systemEnabled,
    bool? quietHoursEnabled,
    String? quietStart,
    String? quietEnd,
    String? timezone,
    bool? digestEnabled,
    String? digestFrequency,
  }) {
    return NotificationPreference(
      pushEnabled: pushEnabled ?? this.pushEnabled,
      emailEnabled: emailEnabled ?? this.emailEnabled,
      smsEnabled: smsEnabled ?? this.smsEnabled,
      inAppEnabled: inAppEnabled ?? this.inAppEnabled,
      likesEnabled: likesEnabled ?? this.likesEnabled,
      commentsEnabled: commentsEnabled ?? this.commentsEnabled,
      followsEnabled: followsEnabled ?? this.followsEnabled,
      mentionsEnabled: mentionsEnabled ?? this.mentionsEnabled,
      giftsEnabled: giftsEnabled ?? this.giftsEnabled,
      ordersEnabled: ordersEnabled ?? this.ordersEnabled,
      livestreamEnabled: livestreamEnabled ?? this.livestreamEnabled,
      systemEnabled: systemEnabled ?? this.systemEnabled,
      quietHoursEnabled: quietHoursEnabled ?? this.quietHoursEnabled,
      quietStart: quietStart ?? this.quietStart,
      quietEnd: quietEnd ?? this.quietEnd,
      timezone: timezone ?? this.timezone,
      digestEnabled: digestEnabled ?? this.digestEnabled,
      digestFrequency: digestFrequency ?? this.digestFrequency,
    );
  }
}
