import 'package:dio/dio.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/network/api_client.dart';
import '../../../../core/network/api_endpoints.dart';
import '../models/order_model.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Abstract contract
// ─────────────────────────────────────────────────────────────────────────────

abstract class EcommerceRemoteDataSource {
  Future<Map<String, dynamic>> getProducts({
    String? category,
    String? cursor,
  });

  Future<Map<String, dynamic>> getProduct(String id);

  Future<Map<String, dynamic>> searchProducts({
    required String query,
    String? cursor,
  });

  Future<CartModel> getCart();

  Future<CartModel> addToCart({
    required String productId,
    required String? variantId,
    required int qty,
  });

  Future<CartModel> updateCartItem({
    required String itemId,
    required int qty,
  });

  Future<CartModel> removeCartItem(String itemId);

  Future<OrderModel> placeOrder({
    required Map<String, dynamic> shippingAddress,
    required String paymentMethod,
  });

  Future<Map<String, dynamic>> getOrders({String? cursor});

  Future<OrderModel> getOrder(String id);

  Future<OrderModel> cancelOrder(String id);
}

// ─────────────────────────────────────────────────────────────────────────────
// Implementation
// ─────────────────────────────────────────────────────────────────────────────

class EcommerceRemoteDataSourceImpl implements EcommerceRemoteDataSource {
  EcommerceRemoteDataSourceImpl({Dio? dio})
      : _dio = dio ?? ApiClient.instance.dio;

  final Dio _dio;

  // ── Products ──────────────────────────────────────────────────────────────

  @override
  Future<Map<String, dynamic>> getProducts({
    String? category,
    String? cursor,
  }) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        ApiEndpoints.products,
        queryParameters: {
          if (category != null && category != 'All') 'category': category,
          if (cursor != null) 'cursor': cursor,
          'limit': 20,
        },
      );
      return _extractData(response);
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<Map<String, dynamic>> getProduct(String id) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        ApiEndpoints.productById(id),
      );
      return _extractData(response);
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<Map<String, dynamic>> searchProducts({
    required String query,
    String? cursor,
  }) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '${ApiEndpoints.products}/search',
        queryParameters: {
          'q': query,
          if (cursor != null) 'cursor': cursor,
          'limit': 20,
        },
      );
      return _extractData(response);
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  // ── Cart ──────────────────────────────────────────────────────────────────

  @override
  Future<CartModel> getCart() async {
    try {
      final response =
          await _dio.get<Map<String, dynamic>>(ApiEndpoints.cart);
      return CartModel.fromJson(_extractData(response));
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<CartModel> addToCart({
    required String productId,
    required String? variantId,
    required int qty,
  }) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '${ApiEndpoints.cart}/items',
        data: {
          'product_id': productId,
          if (variantId != null) 'variant_id': variantId,
          'qty': qty,
        },
      );
      return CartModel.fromJson(_extractData(response));
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<CartModel> updateCartItem({
    required String itemId,
    required int qty,
  }) async {
    try {
      final response = await _dio.put<Map<String, dynamic>>(
        '${ApiEndpoints.cart}/items/$itemId',
        data: {'qty': qty},
      );
      return CartModel.fromJson(_extractData(response));
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<CartModel> removeCartItem(String itemId) async {
    try {
      final response = await _dio.delete<Map<String, dynamic>>(
        '${ApiEndpoints.cart}/items/$itemId',
      );
      return CartModel.fromJson(_extractData(response));
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  // ── Orders ────────────────────────────────────────────────────────────────

  @override
  Future<OrderModel> placeOrder({
    required Map<String, dynamic> shippingAddress,
    required String paymentMethod,
  }) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        ApiEndpoints.orders,
        data: {
          'shipping_address': shippingAddress,
          'payment_method': paymentMethod,
        },
      );
      return OrderModel.fromJson(_extractData(response));
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<Map<String, dynamic>> getOrders({String? cursor}) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        ApiEndpoints.orders,
        queryParameters: {
          if (cursor != null) 'cursor': cursor,
          'limit': 20,
        },
      );
      return _extractData(response);
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<OrderModel> getOrder(String id) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        ApiEndpoints.orderById(id),
      );
      return OrderModel.fromJson(_extractData(response));
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<OrderModel> cancelOrder(String id) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '${ApiEndpoints.orderById(id)}/cancel',
      );
      return OrderModel.fromJson(_extractData(response));
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  Map<String, dynamic> _extractData(Response<Map<String, dynamic>> response) {
    final body = response.data;
    if (body == null) throw const ServerException(message: 'Empty response');
    // Support both { data: {...} } and flat responses.
    if (body.containsKey('data') && body['data'] is Map<String, dynamic>) {
      return body['data'] as Map<String, dynamic>;
    }
    return body;
  }

  Exception _mapDioException(DioException e) {
    final statusCode = e.response?.statusCode;
    if (e.type == DioExceptionType.connectionError ||
        e.type == DioExceptionType.connectionTimeout) {
      return const NetworkException();
    }
    final message = (e.response?.data as Map<String, dynamic>?)?['message']
            as String? ??
        e.message ??
        'Unknown error';
    return ServerException(message: message, statusCode: statusCode);
  }
}
