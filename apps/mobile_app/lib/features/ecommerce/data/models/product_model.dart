import '../../domain/entities/product_entity.dart';

// ─────────────────────────────────────────────────────────────────────────────
// ProductVariantModel
// ─────────────────────────────────────────────────────────────────────────────

class ProductVariantModel extends ProductVariantEntity {
  const ProductVariantModel({
    required super.id,
    required super.name,
    required super.sku,
    required super.price,
    required super.stockQty,
    required super.attributes,
  });

  factory ProductVariantModel.fromJson(Map<String, dynamic> json) {
    return ProductVariantModel(
      id: json['id'] as String,
      name: json['name'] as String,
      sku: json['sku'] as String,
      price: (json['price'] as num).toDouble(),
      stockQty: (json['stock_qty'] as num?)?.toInt() ?? 0,
      attributes: (json['attributes'] as Map<String, dynamic>?)?.map(
            (k, v) => MapEntry(k, v as String),
          ) ??
          {},
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'sku': sku,
      'price': price,
      'stock_qty': stockQty,
      'attributes': attributes,
    };
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// ProductModel
// ─────────────────────────────────────────────────────────────────────────────

class ProductModel extends ProductEntity {
  const ProductModel({
    required super.id,
    required super.sellerId,
    required super.sellerName,
    required super.sellerAvatarUrl,
    required super.name,
    required super.description,
    required super.price,
    super.discountPrice,
    required super.images,
    required super.variants,
    required super.category,
    required super.rating,
    required super.reviewCount,
    required super.soldCount,
    required super.inStock,
    required super.tags,
  });

  factory ProductModel.fromJson(Map<String, dynamic> json) {
    return ProductModel(
      id: json['id'] as String,
      sellerId: json['seller_id'] as String,
      sellerName: json['seller_name'] as String,
      sellerAvatarUrl: json['seller_avatar_url'] as String? ?? '',
      name: json['name'] as String,
      description: json['description'] as String? ?? '',
      price: (json['price'] as num).toDouble(),
      discountPrice: json['discount_price'] != null
          ? (json['discount_price'] as num).toDouble()
          : null,
      images: (json['images'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      variants: (json['variants'] as List<dynamic>?)
              ?.map((v) =>
                  ProductVariantModel.fromJson(v as Map<String, dynamic>))
              .toList() ??
          [],
      category: json['category'] as String? ?? 'Other',
      rating: (json['rating'] as num?)?.toDouble() ?? 0.0,
      reviewCount: (json['review_count'] as num?)?.toInt() ?? 0,
      soldCount: (json['sold_count'] as num?)?.toInt() ?? 0,
      inStock: json['in_stock'] as bool? ?? true,
      tags: (json['tags'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'seller_id': sellerId,
      'seller_name': sellerName,
      'seller_avatar_url': sellerAvatarUrl,
      'name': name,
      'description': description,
      'price': price,
      'discount_price': discountPrice,
      'images': images,
      'variants': variants
          .map((v) => (v as ProductVariantModel).toJson())
          .toList(),
      'category': category,
      'rating': rating,
      'review_count': reviewCount,
      'sold_count': soldCount,
      'in_stock': inStock,
      'tags': tags,
    };
  }
}
