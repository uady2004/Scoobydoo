class VideoEntity {
  const VideoEntity({
    required this.id,
    required this.creatorId,
    required this.creatorUsername,
    required this.creatorAvatarUrl,
    required this.videoUrl,
    this.hlsUrl,
    required this.thumbnailUrl,
    required this.description,
    required this.likeCount,
    required this.commentCount,
    required this.shareCount,
    required this.viewCount,
    required this.isLiked,
    required this.isBookmarked,
    required this.isFollowing,
    this.soundTitle,
    this.soundId,
    this.duration,
    required this.createdAt,
    this.tags = const [],
  });

  final String id;
  final String creatorId;
  final String creatorUsername;
  final String creatorAvatarUrl;
  final String videoUrl;
  final String? hlsUrl;
  final String thumbnailUrl;
  final String description;
  final int likeCount;
  final int commentCount;
  final int shareCount;
  final int viewCount;
  final bool isLiked;
  final bool isBookmarked;
  final bool isFollowing;
  final String? soundTitle;
  final String? soundId;
  final int? duration;
  final DateTime createdAt;
  final List<String> tags;

  VideoEntity copyWith({
    bool? isLiked,
    bool? isBookmarked,
    bool? isFollowing,
    int? likeCount,
    int? commentCount,
    int? shareCount,
  }) =>
      VideoEntity(
        id: id,
        creatorId: creatorId,
        creatorUsername: creatorUsername,
        creatorAvatarUrl: creatorAvatarUrl,
        videoUrl: videoUrl,
        hlsUrl: hlsUrl,
        thumbnailUrl: thumbnailUrl,
        description: description,
        likeCount: likeCount ?? this.likeCount,
        commentCount: commentCount ?? this.commentCount,
        shareCount: shareCount ?? this.shareCount,
        viewCount: viewCount,
        isLiked: isLiked ?? this.isLiked,
        isBookmarked: isBookmarked ?? this.isBookmarked,
        isFollowing: isFollowing ?? this.isFollowing,
        soundTitle: soundTitle,
        soundId: soundId,
        duration: duration,
        createdAt: createdAt,
        tags: tags,
      );
}
