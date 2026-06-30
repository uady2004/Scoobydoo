import '../../domain/entities/feed_item_entity.dart';

/// Data-layer model — adds JSON serialisation on top of the entity.
class FeedItemModel extends FeedItemEntity {
  const FeedItemModel({
    required super.videoId,
    required super.videoUrl,
    required super.thumbnailUrl,
    required super.hlsUrl,
    required super.title,
    required super.description,
    required super.hashtags,
    required super.soundTitle,
    required super.soundArtist,
    required super.creatorId,
    required super.creatorUsername,
    required super.creatorAvatarUrl,
    required super.isCreatorVerified,
    required super.isFollowing,
    required super.likeCount,
    required super.commentCount,
    required super.shareCount,
    required super.bookmarkCount,
    required super.viewCount,
    required super.isLiked,
    required super.isBookmarked,
    required super.duration,
    required super.createdAt,
  });

  factory FeedItemModel.fromJson(Map<String, dynamic> json) {
    return FeedItemModel(
      videoId: json['video_id'] as String,
      videoUrl: json['video_url'] as String,
      thumbnailUrl: json['thumbnail_url'] as String,
      hlsUrl: json['hls_url'] as String,
      title: json['title'] as String? ?? '',
      description: json['description'] as String? ?? '',
      hashtags: (json['hashtags'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      soundTitle: json['sound_title'] as String? ?? 'Original Sound',
      soundArtist: json['sound_artist'] as String? ?? '',
      creatorId: json['creator_id'] as String,
      creatorUsername: json['creator_username'] as String,
      creatorAvatarUrl: json['creator_avatar_url'] as String? ?? '',
      isCreatorVerified: json['is_creator_verified'] as bool? ?? false,
      isFollowing: json['is_following'] as bool? ?? false,
      likeCount: (json['like_count'] as num?)?.toInt() ?? 0,
      commentCount: (json['comment_count'] as num?)?.toInt() ?? 0,
      shareCount: (json['share_count'] as num?)?.toInt() ?? 0,
      bookmarkCount: (json['bookmark_count'] as num?)?.toInt() ?? 0,
      viewCount: (json['view_count'] as num?)?.toInt() ?? 0,
      isLiked: json['is_liked'] as bool? ?? false,
      isBookmarked: json['is_bookmarked'] as bool? ?? false,
      duration: (json['duration'] as num?)?.toInt() ?? 0,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'video_id': videoId,
      'video_url': videoUrl,
      'thumbnail_url': thumbnailUrl,
      'hls_url': hlsUrl,
      'title': title,
      'description': description,
      'hashtags': hashtags,
      'sound_title': soundTitle,
      'sound_artist': soundArtist,
      'creator_id': creatorId,
      'creator_username': creatorUsername,
      'creator_avatar_url': creatorAvatarUrl,
      'is_creator_verified': isCreatorVerified,
      'is_following': isFollowing,
      'like_count': likeCount,
      'comment_count': commentCount,
      'share_count': shareCount,
      'bookmark_count': bookmarkCount,
      'view_count': viewCount,
      'is_liked': isLiked,
      'is_bookmarked': isBookmarked,
      'duration': duration,
      'created_at': createdAt.toIso8601String(),
    };
  }

  /// Converts the model to its pure domain entity counterpart.
  FeedItemEntity toEntity() => this;
}
