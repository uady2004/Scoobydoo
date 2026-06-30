class GiftEntity {
  const GiftEntity({
    required this.id,
    required this.name,
    required this.coinCost,
    required this.animationKey,
    required this.previewImageUrl,
    required this.category,
  });
  final String id;
  final String name;
  final int coinCost;
  final String animationKey;
  final String previewImageUrl;
  final String category;
}
