// Livestream domain models – mirrors the Go backend response shapes.

enum StreamStatus { pending, live, ended, banned }

enum BattleStatus { pending, active, ended, declined }

enum PollStatus { active, closed }

class LiveStream {
  final String id;
  final String userId;
  final String title;
  final String description;
  final String? rtmpKey;
  final String hlsPlaylistUrl;
  final String thumbnailUrl;
  final StreamStatus status;
  final int viewerCount;
  final int peakViewerCount;
  final int totalGiftCoins;
  final String categoryId;
  final List<String> tags;
  final bool isRecorded;
  final String? recordingUrl;
  final String language;
  final bool ageRestricted;
  final bool allowComments;
  final String? pkBattleId;
  final DateTime? startedAt;
  final DateTime? endedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  // Presenter helpers (populated from a separate user fetch or join-payload).
  final String? hostUsername;
  final String? hostAvatarUrl;
  final int hostFollowerCount;

  const LiveStream({
    required this.id,
    required this.userId,
    required this.title,
    this.description = '',
    this.rtmpKey,
    required this.hlsPlaylistUrl,
    this.thumbnailUrl = '',
    required this.status,
    this.viewerCount = 0,
    this.peakViewerCount = 0,
    this.totalGiftCoins = 0,
    this.categoryId = '',
    this.tags = const [],
    this.isRecorded = false,
    this.recordingUrl,
    this.language = 'en',
    this.ageRestricted = false,
    this.allowComments = true,
    this.pkBattleId,
    this.startedAt,
    this.endedAt,
    required this.createdAt,
    required this.updatedAt,
    this.hostUsername,
    this.hostAvatarUrl,
    this.hostFollowerCount = 0,
  });

  factory LiveStream.fromJson(Map<String, dynamic> json) {
    return LiveStream(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      title: json['title'] as String? ?? '',
      description: json['description'] as String? ?? '',
      rtmpKey: json['rtmp_key'] as String?,
      hlsPlaylistUrl: json['hls_playlist_url'] as String? ?? '',
      thumbnailUrl: json['thumbnail_url'] as String? ?? '',
      status: _parseStatus(json['status'] as String? ?? 'pending'),
      viewerCount: json['viewer_count'] as int? ?? 0,
      peakViewerCount: json['peak_viewer_count'] as int? ?? 0,
      totalGiftCoins: json['total_gift_coins'] as int? ?? 0,
      categoryId: json['category_id'] as String? ?? '',
      tags: (json['tags'] as List<dynamic>?)?.cast<String>() ?? [],
      isRecorded: json['is_recorded'] as bool? ?? false,
      recordingUrl: json['recording_url'] as String?,
      language: json['language'] as String? ?? 'en',
      ageRestricted: json['age_restricted'] as bool? ?? false,
      allowComments: json['allow_comments'] as bool? ?? true,
      pkBattleId: json['pk_battle_id'] as String?,
      startedAt: json['started_at'] != null
          ? DateTime.parse(json['started_at'] as String)
          : null,
      endedAt: json['ended_at'] != null
          ? DateTime.parse(json['ended_at'] as String)
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
      hostUsername: json['host_username'] as String?,
      hostAvatarUrl: json['host_avatar_url'] as String?,
      hostFollowerCount: json['host_follower_count'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'user_id': userId,
        'title': title,
        'description': description,
        'hls_playlist_url': hlsPlaylistUrl,
        'thumbnail_url': thumbnailUrl,
        'status': status.name,
        'viewer_count': viewerCount,
        'category_id': categoryId,
        'tags': tags,
        'language': language,
        'age_restricted': ageRestricted,
        'allow_comments': allowComments,
      };

  LiveStream copyWith({
    int? viewerCount,
    int? peakViewerCount,
    int? totalGiftCoins,
    StreamStatus? status,
    String? pkBattleId,
    String? hlsPlaylistUrl,
    String? recordingUrl,
  }) {
    return LiveStream(
      id: id,
      userId: userId,
      title: title,
      description: description,
      rtmpKey: rtmpKey,
      hlsPlaylistUrl: hlsPlaylistUrl ?? this.hlsPlaylistUrl,
      thumbnailUrl: thumbnailUrl,
      status: status ?? this.status,
      viewerCount: viewerCount ?? this.viewerCount,
      peakViewerCount: peakViewerCount ?? this.peakViewerCount,
      totalGiftCoins: totalGiftCoins ?? this.totalGiftCoins,
      categoryId: categoryId,
      tags: tags,
      isRecorded: isRecorded,
      recordingUrl: recordingUrl ?? this.recordingUrl,
      language: language,
      ageRestricted: ageRestricted,
      allowComments: allowComments,
      pkBattleId: pkBattleId ?? this.pkBattleId,
      startedAt: startedAt,
      endedAt: endedAt,
      createdAt: createdAt,
      updatedAt: DateTime.now(),
      hostUsername: hostUsername,
      hostAvatarUrl: hostAvatarUrl,
      hostFollowerCount: hostFollowerCount,
    );
  }

  static StreamStatus _parseStatus(String s) {
    switch (s) {
      case 'live':
        return StreamStatus.live;
      case 'ended':
        return StreamStatus.ended;
      case 'banned':
        return StreamStatus.banned;
      default:
        return StreamStatus.pending;
    }
  }
}

// ---------------------------------------------------------------------------

class LiveMessage {
  final String id;
  final String streamId;
  final String userId;
  final String username;
  final String avatarUrl;
  final String content;
  final String type; // "text" | "emoji" | "sticker" | "system"
  final bool isPinned;
  final bool isDeleted;
  final Map<String, int> reactions;
  final String? replyToId;
  final DateTime createdAt;

  const LiveMessage({
    required this.id,
    required this.streamId,
    required this.userId,
    required this.username,
    this.avatarUrl = '',
    required this.content,
    this.type = 'text',
    this.isPinned = false,
    this.isDeleted = false,
    this.reactions = const {},
    this.replyToId,
    required this.createdAt,
  });

  factory LiveMessage.fromJson(Map<String, dynamic> json) {
    return LiveMessage(
      id: json['id'] as String,
      streamId: json['stream_id'] as String,
      userId: json['user_id'] as String,
      username: json['username'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String? ?? '',
      content: json['content'] as String? ?? '',
      type: json['type'] as String? ?? 'text',
      isPinned: json['is_pinned'] as bool? ?? false,
      isDeleted: json['is_deleted'] as bool? ?? false,
      reactions: (json['reactions'] as Map<String, dynamic>?)
              ?.map((k, v) => MapEntry(k, v as int)) ??
          {},
      replyToId: json['reply_to_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}

// ---------------------------------------------------------------------------

class GiftType {
  final String id;
  final String name;
  final String description;
  final String iconUrl;
  final String animationUrl;
  final int coinPrice;
  final String category; // "basic" | "premium" | "limited"

  const GiftType({
    required this.id,
    required this.name,
    this.description = '',
    required this.iconUrl,
    required this.animationUrl,
    required this.coinPrice,
    this.category = 'basic',
  });

  factory GiftType.fromJson(Map<String, dynamic> json) {
    return GiftType(
      id: json['id'] as String,
      name: json['name'] as String,
      description: json['description'] as String? ?? '',
      iconUrl: json['icon_url'] as String? ?? '',
      animationUrl: json['animation_url'] as String? ?? '',
      coinPrice: json['coin_price'] as int? ?? 0,
      category: json['category'] as String? ?? 'basic',
    );
  }
}

class GiftSentEvent {
  final String giftName;
  final String animationUrl;
  final String senderName;
  final int quantity;
  final int comboCount;
  final int totalCoins;

  const GiftSentEvent({
    required this.giftName,
    required this.animationUrl,
    required this.senderName,
    required this.quantity,
    this.comboCount = 1,
    required this.totalCoins,
  });

  factory GiftSentEvent.fromJson(Map<String, dynamic> json) {
    return GiftSentEvent(
      giftName: json['gift_name'] as String,
      animationUrl: json['animation_url'] as String? ?? '',
      senderName: json['sender_name'] as String? ?? '',
      quantity: json['quantity'] as int? ?? 1,
      comboCount: json['combo_count'] as int? ?? 1,
      totalCoins: json['total_coins'] as int? ?? 0,
    );
  }
}

// ---------------------------------------------------------------------------

class PKBattle {
  final String id;
  final String initiatorId;
  final String initiatorName;
  final String targetId;
  final String targetName;
  final String streamId;
  final String targetStreamId;
  final BattleStatus status;
  final int initiatorScore;
  final int targetScore;
  final String? winnerId;
  final int durationSecs;
  final DateTime? startedAt;
  final DateTime? endedAt;

  const PKBattle({
    required this.id,
    required this.initiatorId,
    required this.initiatorName,
    required this.targetId,
    required this.targetName,
    required this.streamId,
    required this.targetStreamId,
    required this.status,
    this.initiatorScore = 0,
    this.targetScore = 0,
    this.winnerId,
    this.durationSecs = 60,
    this.startedAt,
    this.endedAt,
  });

  factory PKBattle.fromJson(Map<String, dynamic> json) {
    return PKBattle(
      id: json['id'] as String,
      initiatorId: json['initiator_id'] as String,
      initiatorName: json['initiator_name'] as String? ?? '',
      targetId: json['target_id'] as String,
      targetName: json['target_name'] as String? ?? '',
      streamId: json['stream_id'] as String,
      targetStreamId: json['target_stream_id'] as String,
      status: _parseBattleStatus(json['status'] as String? ?? 'pending'),
      initiatorScore: json['initiator_score'] as int? ?? 0,
      targetScore: json['target_score'] as int? ?? 0,
      winnerId: json['winner_id'] as String?,
      durationSecs: json['duration_secs'] as int? ?? 60,
      startedAt: json['started_at'] != null
          ? DateTime.parse(json['started_at'] as String)
          : null,
      endedAt: json['ended_at'] != null
          ? DateTime.parse(json['ended_at'] as String)
          : null,
    );
  }

  PKBattle copyWith({int? initiatorScore, int? targetScore, BattleStatus? status, String? winnerId}) {
    return PKBattle(
      id: id,
      initiatorId: initiatorId,
      initiatorName: initiatorName,
      targetId: targetId,
      targetName: targetName,
      streamId: streamId,
      targetStreamId: targetStreamId,
      status: status ?? this.status,
      initiatorScore: initiatorScore ?? this.initiatorScore,
      targetScore: targetScore ?? this.targetScore,
      winnerId: winnerId ?? this.winnerId,
      durationSecs: durationSecs,
      startedAt: startedAt,
      endedAt: endedAt,
    );
  }

  static BattleStatus _parseBattleStatus(String s) {
    switch (s) {
      case 'active':
        return BattleStatus.active;
      case 'ended':
        return BattleStatus.ended;
      case 'declined':
        return BattleStatus.declined;
      default:
        return BattleStatus.pending;
    }
  }
}

// ---------------------------------------------------------------------------

class Poll {
  final String id;
  final String streamId;
  final String creatorId;
  final String question;
  final List<PollOption> options;
  final PollStatus status;
  final int durationSecs;
  final int totalVotes;
  final DateTime createdAt;
  final DateTime? closedAt;

  const Poll({
    required this.id,
    required this.streamId,
    required this.creatorId,
    required this.question,
    required this.options,
    required this.status,
    this.durationSecs = 60,
    this.totalVotes = 0,
    required this.createdAt,
    this.closedAt,
  });

  factory Poll.fromJson(Map<String, dynamic> json) {
    final optionsRaw = json['options'] as List<dynamic>? ?? [];
    return Poll(
      id: json['id'] as String,
      streamId: json['stream_id'] as String,
      creatorId: json['creator_id'] as String,
      question: json['question'] as String,
      options: optionsRaw
          .map((e) => PollOption.fromJson(e as Map<String, dynamic>))
          .toList(),
      status: (json['status'] as String?) == 'closed'
          ? PollStatus.closed
          : PollStatus.active,
      durationSecs: json['duration_secs'] as int? ?? 60,
      totalVotes: json['total_votes'] as int? ?? 0,
      createdAt: DateTime.parse(json['created_at'] as String),
      closedAt: json['closed_at'] != null
          ? DateTime.parse(json['closed_at'] as String)
          : null,
    );
  }

  Poll copyWith({List<PollOption>? options, int? totalVotes, PollStatus? status}) {
    return Poll(
      id: id,
      streamId: streamId,
      creatorId: creatorId,
      question: question,
      options: options ?? this.options,
      status: status ?? this.status,
      durationSecs: durationSecs,
      totalVotes: totalVotes ?? this.totalVotes,
      createdAt: createdAt,
      closedAt: closedAt,
    );
  }
}

class PollOption {
  final String id;
  final String pollId;
  final String text;
  final int voteCount;
  final double percentage;

  const PollOption({
    required this.id,
    required this.pollId,
    required this.text,
    this.voteCount = 0,
    this.percentage = 0.0,
  });

  factory PollOption.fromJson(Map<String, dynamic> json) {
    return PollOption(
      id: json['id'] as String,
      pollId: json['poll_id'] as String,
      text: json['text'] as String,
      voteCount: json['vote_count'] as int? ?? 0,
      percentage: (json['percentage'] as num?)?.toDouble() ?? 0.0,
    );
  }
}

// ---------------------------------------------------------------------------

class StartStreamRequest {
  final String title;
  final String description;
  final String categoryId;
  final List<String> tags;
  final bool allowComments;
  final bool ageRestricted;

  const StartStreamRequest({
    required this.title,
    this.description = '',
    this.categoryId = '',
    this.tags = const [],
    this.allowComments = true,
    this.ageRestricted = false,
  });

  Map<String, dynamic> toJson() => {
        'title': title,
        'description': description,
        'category_id': categoryId,
        'tags': tags,
        'allow_comments': allowComments,
        'age_restricted': ageRestricted,
      };
}

class SendGiftRequest {
  final String streamId;
  final String giftTypeId;
  final int quantity;

  const SendGiftRequest({
    required this.streamId,
    required this.giftTypeId,
    this.quantity = 1,
  });

  Map<String, dynamic> toJson() => {
        'stream_id': streamId,
        'gift_type_id': giftTypeId,
        'quantity': quantity,
      };
}

class CreatePollRequest {
  final String streamId;
  final String question;
  final List<String> options;
  final int durationSecs;

  const CreatePollRequest({
    required this.streamId,
    required this.question,
    required this.options,
    this.durationSecs = 60,
  });

  Map<String, dynamic> toJson() => {
        'stream_id': streamId,
        'question': question,
        'options': options,
        'duration_secs': durationSecs,
      };
}
