class LikeEntity {
  const LikeEntity({
    required this.videoId,
    required this.userId,
    required this.isLiked,
    required this.likeCount,
  });
  final String videoId;
  final String userId;
  final bool isLiked;
  final int likeCount;
  LikeEntity copyWith({bool? isLiked, int? likeCount}) => LikeEntity(
    videoId: videoId, userId: userId,
    isLiked: isLiked ?? this.isLiked,
    likeCount: likeCount ?? this.likeCount,
  );
}
