import 'package:fpdart/fpdart.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/error/failures.dart';
import '../../domain/entities/order_entity.dart';
import '../../domain/entities/product_entity.dart';
import '../../domain/repositories/ecommerce_repository.dart';
import '../datasources/ecommerce_remote_datasource.dart';
import '../models/order_model.dart';
import '../models/product_model.dart';

class EcommerceRepositoryImpl implements EcommerceRepository {
  const EcommerceRepositoryImpl(this._remote);
  final EcommerceRemoteDataSource _remote;

  // ── Products ──────────────────────────────────────────────────────────────

  @override
  Future<Either<Failure, PaginatedProducts>> getProducts({
    String? category,
    String? cursor,
  }) async {
    try {
      final json = await _remote.getProducts(category: category, cursor: cursor);
      final items = (json['items'] as List<dynamic>? ?? [])
          .map((i) => ProductModel.fromJson(i as Map<String, dynamic>))
          .toList();
      return right(PaginatedProducts(
        items: items,
        nextCursor: json['next_cursor'] as String?,
      ));
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, ProductEntity>> getProduct(String id) async {
    try {
      final json = await _remote.getProduct(id);
      return right(ProductModel.fromJson(json));
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, PaginatedProducts>> searchProducts({
    required String query,
    String? cursor,
  }) async {
    try {
      final json =
          await _remote.searchProducts(query: query, cursor: cursor);
      final items = (json['items'] as List<dynamic>? ?? [])
          .map((i) => ProductModel.fromJson(i as Map<String, dynamic>))
          .toList();
      return right(PaginatedProducts(
        items: items,
        nextCursor: json['next_cursor'] as String?,
      ));
    } catch (e) {
      return left(_map(e));
    }
  }

  // ── Cart ──────────────────────────────────────────────────────────────────

  @override
  Future<Either<Failure, CartEntity>> getCart() async {
    try {
      return right(await _remote.getCart());
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, CartEntity>> addToCart({
    required String productId,
    required String? variantId,
    required int qty,
  }) async {
    try {
      return right(await _remote.addToCart(
          productId: productId, variantId: variantId, qty: qty));
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, CartEntity>> updateCartItem({
    required String itemId,
    required int qty,
  }) async {
    try {
      return right(await _remote.updateCartItem(itemId: itemId, qty: qty));
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, CartEntity>> removeCartItem(String itemId) async {
    try {
      return right(await _remote.removeCartItem(itemId));
    } catch (e) {
      return left(_map(e));
    }
  }

  // ── Orders ────────────────────────────────────────────────────────────────

  @override
  Future<Either<Failure, OrderEntity>> placeOrder({
    required BuyerInfoEntity shippingAddress,
    required String paymentMethod,
  }) async {
    try {
      final addressMap = {
        'name': shippingAddress.name,
        'phone': shippingAddress.phone,
        'address_line1': shippingAddress.addressLine1,
        'address_line2': shippingAddress.addressLine2,
        'city': shippingAddress.city,
        'state': shippingAddress.state,
        'postal_code': shippingAddress.postalCode,
        'country': shippingAddress.country,
      };
      return right(await _remote.placeOrder(
        shippingAddress: addressMap,
        paymentMethod: paymentMethod,
      ));
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, PaginatedOrders>> getOrders({
    String? cursor,
  }) async {
    try {
      final json = await _remote.getOrders(cursor: cursor);
      final items = (json['items'] as List<dynamic>? ?? [])
          .map((i) => OrderModel.fromJson(i as Map<String, dynamic>))
          .toList();
      return right(PaginatedOrders(
        items: items,
        nextCursor: json['next_cursor'] as String?,
      ));
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, OrderEntity>> getOrder(String id) async {
    try {
      return right(await _remote.getOrder(id));
    } catch (e) {
      return left(_map(e));
    }
  }

  @override
  Future<Either<Failure, OrderEntity>> cancelOrder(String id) async {
    try {
      return right(await _remote.cancelOrder(id));
    } catch (e) {
      return left(_map(e));
    }
  }

  // ── Error mapping ─────────────────────────────────────────────────────────

  Failure _map(Object e) {
    if (e is ServerException) {
      return ServerFailure(message: e.message, statusCode: e.statusCode);
    }
    if (e is NetworkException) return const NetworkFailure();
    if (e is AuthException) {
      return AuthFailure(message: e.message, statusCode: e.statusCode);
    }
    return UnexpectedFailure(message: e.toString());
  }
}
