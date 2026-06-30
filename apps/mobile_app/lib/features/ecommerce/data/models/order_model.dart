import '../../domain/entities/order_entity.dart';
import 'product_model.dart';

// ─────────────────────────────────────────────────────────────────────────────
// BuyerInfoModel
// ─────────────────────────────────────────────────────────────────────────────

class BuyerInfoModel extends BuyerInfoEntity {
  const BuyerInfoModel({
    required super.name,
    required super.phone,
    required super.addressLine1,
    super.addressLine2,
    required super.city,
    required super.state,
    required super.postalCode,
    required super.country,
  });

  factory BuyerInfoModel.fromJson(Map<String, dynamic> json) {
    return BuyerInfoModel(
      name: json['name'] as String,
      phone: json['phone'] as String,
      addressLine1: json['address_line1'] as String,
      addressLine2: json['address_line2'] as String?,
      city: json['city'] as String,
      state: json['state'] as String,
      postalCode: json['postal_code'] as String,
      country: json['country'] as String,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'name': name,
      'phone': phone,
      'address_line1': addressLine1,
      'address_line2': addressLine2,
      'city': city,
      'state': state,
      'postal_code': postalCode,
      'country': country,
    };
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// OrderItemModel
// ─────────────────────────────────────────────────────────────────────────────

class OrderItemModel extends OrderItemEntity {
  const OrderItemModel({
    required super.product,
    super.variant,
    required super.qty,
    required super.unitPrice,
  });

  factory OrderItemModel.fromJson(Map<String, dynamic> json) {
    final productJson = json['product'] as Map<String, dynamic>;
    final variantJson = json['variant'] as Map<String, dynamic>?;
    return OrderItemModel(
      product: ProductModel.fromJson(productJson),
      variant: variantJson != null
          ? ProductVariantModel.fromJson(variantJson)
          : null,
      qty: (json['qty'] as num).toInt(),
      unitPrice: (json['unit_price'] as num).toDouble(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'product': (product as ProductModel).toJson(),
      'variant':
          variant != null ? (variant as ProductVariantModel).toJson() : null,
      'qty': qty,
      'unit_price': unitPrice,
    };
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// OrderModel
// ─────────────────────────────────────────────────────────────────────────────

class OrderModel extends OrderEntity {
  const OrderModel({
    required super.id,
    required super.buyerInfo,
    required super.items,
    required super.status,
    required super.subtotal,
    required super.shippingFee,
    required super.total,
    super.trackingNumber,
    super.courierName,
    required super.paymentMethod,
    required super.createdAt,
  });

  factory OrderModel.fromJson(Map<String, dynamic> json) {
    return OrderModel(
      id: json['id'] as String,
      buyerInfo: BuyerInfoModel.fromJson(
          json['buyer_info'] as Map<String, dynamic>),
      items: (json['items'] as List<dynamic>)
          .map((i) => OrderItemModel.fromJson(i as Map<String, dynamic>))
          .toList(),
      status: OrderStatus.fromString(json['status'] as String),
      subtotal: (json['subtotal'] as num).toDouble(),
      shippingFee: (json['shipping_fee'] as num).toDouble(),
      total: (json['total'] as num).toDouble(),
      trackingNumber: json['tracking_number'] as String?,
      courierName: json['courier_name'] as String?,
      paymentMethod: json['payment_method'] as String? ?? 'card',
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'buyer_info': (buyerInfo as BuyerInfoModel).toJson(),
      'items': items
          .map((i) => (i as OrderItemModel).toJson())
          .toList(),
      'status': status.name,
      'subtotal': subtotal,
      'shipping_fee': shippingFee,
      'total': total,
      'tracking_number': trackingNumber,
      'courier_name': courierName,
      'payment_method': paymentMethod,
      'created_at': createdAt.toIso8601String(),
    };
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// CartItemModel
// ─────────────────────────────────────────────────────────────────────────────

class CartItemModel extends CartItemEntity {
  const CartItemModel({
    required super.id,
    required super.productId,
    super.variantId,
    required super.qty,
    required super.product,
  });

  factory CartItemModel.fromJson(Map<String, dynamic> json) {
    return CartItemModel(
      id: json['id'] as String,
      productId: json['product_id'] as String,
      variantId: json['variant_id'] as String?,
      qty: (json['qty'] as num).toInt(),
      product:
          ProductModel.fromJson(json['product'] as Map<String, dynamic>),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'product_id': productId,
      'variant_id': variantId,
      'qty': qty,
      'product': (product as ProductModel).toJson(),
    };
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// CartModel
// ─────────────────────────────────────────────────────────────────────────────

class CartModel extends CartEntity {
  const CartModel({required super.items});

  factory CartModel.fromJson(Map<String, dynamic> json) {
    return CartModel(
      items: (json['items'] as List<dynamic>?)
              ?.map((i) =>
                  CartItemModel.fromJson(i as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'items': items
          .map((i) => (i as CartItemModel).toJson())
          .toList(),
    };
  }
}
