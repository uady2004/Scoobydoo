import 'package:equatable/equatable.dart';

/// Pure domain entity — no JSON, no framework dependencies.
class FeedItemEntity extends Equatable {
  const FeedItemEntity({
    required this.videoId,
    required this.videoUrl,
    required this.thumbnailUrl,
    required this.hlsUrl,
    required this.title,
    required this.description,
    required this.hashtags,
    required this.soundTitle,
    required this.soundArtist,
    required this.creatorId,
    required this.creatorUsername,
    required this.creatorAvatarUrl,
    required this.isCreatorVerified,
    required this.isFollowing,
    required this.likeCount,
    required this.commentCount,
    required this.shareCount,
    required this.bookmarkCount,
    required this.viewCount,
    required this.isLiked,
    required this.isBookmarked,
    required this.duration,
    required this.createdAt,
  });

  final String videoId;
  final String videoUrl;
  final String thumbnailUrl;
  final String hlsUrl;
  final String title;
  final String description;
  final List<String> hashtags;
  final String soundTitle;
  final String soundArtist;
  final String creatorId;
  final String creatorUsername;
  final String creatorAvatarUrl;
  final bool isCreatorVerified;
  final bool isFollowing;
  final int likeCount;
  final int commentCount;
  final int shareCount;
  final int bookmarkCount;
  final int viewCount;
  final bool isLiked;
  final bool isBookmarked;

  /// Video duration in seconds.
  final int duration;
  final DateTime createdAt;

  FeedItemEntity copyWith({
    String? videoId,
    String? videoUrl,
    String? thumbnailUrl,
    String? hlsUrl,
    String? title,
    String? description,
    List<String>? hashtags,
    String? soundTitle,
    String? soundArtist,
    String? creatorId,
    String? creatorUsername,
    String? creatorAvatarUrl,
    bool? isCreatorVerified,
    bool? isFollowing,
    int? likeCount,
    int? commentCount,
    int? shareCount,
    int? bookmarkCount,
    int? viewCount,
    bool? isLiked,
    bool? isBookmarked,
    int? duration,
    DateTime? createdAt,
  }) {
    return FeedItemEntity(
      videoId: videoId ?? this.videoId,
      videoUrl: videoUrl ?? this.videoUrl,
      thumbnailUrl: thumbnailUrl ?? this.thumbnailUrl,
      hlsUrl: hlsUrl ?? this.hlsUrl,
      title: title ?? this.title,
      description: description ?? this.description,
      hashtags: hashtags ?? this.hashtags,
      soundTitle: soundTitle ?? this.soundTitle,
      soundArtist: soundArtist ?? this.soundArtist,
      creatorId: creatorId ?? this.creatorId,
      creatorUsername: creatorUsername ?? this.creatorUsername,
      creatorAvatarUrl: creatorAvatarUrl ?? this.creatorAvatarUrl,
      isCreatorVerified: isCreatorVerified ?? this.isCreatorVerified,
      isFollowing: isFollowing ?? this.isFollowing,
      likeCount: likeCount ?? this.likeCount,
      commentCount: commentCount ?? this.commentCount,
      shareCount: shareCount ?? this.shareCount,
      bookmarkCount: bookmarkCount ?? this.bookmarkCount,
      viewCount: viewCount ?? this.viewCount,
      isLiked: isLiked ?? this.isLiked,
      isBookmarked: isBookmarked ?? this.isBookmarked,
      duration: duration ?? this.duration,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  List<Object?> get props => [
        videoId,
        videoUrl,
        thumbnailUrl,
        hlsUrl,
        title,
        description,
        hashtags,
        soundTitle,
        soundArtist,
        creatorId,
        creatorUsername,
        creatorAvatarUrl,
        isCreatorVerified,
        isFollowing,
        likeCount,
        commentCount,
        shareCount,
        bookmarkCount,
        viewCount,
        isLiked,
        isBookmarked,
        duration,
        createdAt,
      ];
}
