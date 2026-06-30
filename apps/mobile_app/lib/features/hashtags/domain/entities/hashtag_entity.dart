class HashtagEntity {
  const HashtagEntity({
    required this.tag,
    required this.videoCount,
    required this.viewCount,
    this.coverUrl,
    this.isTrending = false,
    this.description,
  });
  final String tag;
  final int videoCount;
  final int viewCount;
  final String? coverUrl;
  final bool isTrending;
  final String? description;
}
