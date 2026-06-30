class BookmarkEntity {
  const BookmarkEntity({
    required this.id,
    required this.videoId,
    required this.userId,
    required this.createdAt,
  });
  final String id;
  final String videoId;
  final String userId;
  final DateTime createdAt;
}
