import 'package:dio/dio.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/network/api_client.dart';
import '../models/wallet_model.dart';

abstract class WalletRemoteDataSource {
  Future<WalletModel> getBalance();
  Future<List<CoinPackageModel>> getCoinPackages();
  Future<List<TransactionModel>> getTransactions({String? cursor});
  Future<Map<String, dynamic>> createPaymentIntent(String packageId);
  Future<WalletModel> confirmPurchase(String paymentIntentId);
  Future<void> withdraw({required int amount, required String method});
}

class WalletRemoteDataSourceImpl implements WalletRemoteDataSource {
  WalletRemoteDataSourceImpl({Dio? dio})
      : _dio = dio ?? ApiClient.instance.dio;

  final Dio _dio;

  @override
  Future<WalletModel> getBalance() async {
    try {
      final response =
          await _dio.get<Map<String, dynamic>>('/wallet/balance');
      return WalletModel.fromJson(response.data!);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            'Failed to fetch balance',
        statusCode: e.response?.statusCode,
      );
    }
  }

  @override
  Future<List<CoinPackageModel>> getCoinPackages() async {
    try {
      final response =
          await _dio.get<Map<String, dynamic>>('/wallet/packages');
      final data = response.data!;
      final items = data['packages'] as List<dynamic>? ?? [];
      return items
          .map((e) =>
              CoinPackageModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            'Failed to fetch coin packages',
        statusCode: e.response?.statusCode,
      );
    }
  }

  @override
  Future<List<TransactionModel>> getTransactions({String? cursor}) async {
    try {
      final params = <String, dynamic>{};
      if (cursor != null) params['cursor'] = cursor;

      final response = await _dio.get<Map<String, dynamic>>(
        '/wallet/transactions',
        queryParameters: params,
      );
      final data = response.data!;
      final items = data['transactions'] as List<dynamic>? ?? [];
      return items
          .map((e) =>
              TransactionModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            'Failed to fetch transactions',
        statusCode: e.response?.statusCode,
      );
    }
  }

  @override
  Future<Map<String, dynamic>> createPaymentIntent(
      String packageId) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/wallet/buy-coins',
        data: {'package_id': packageId},
      );
      return response.data!;
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            'Failed to create payment intent',
        statusCode: e.response?.statusCode,
      );
    }
  }

  @override
  Future<WalletModel> confirmPurchase(String paymentIntentId) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/wallet/buy-coins',
        data: {'payment_intent_id': paymentIntentId},
      );
      return WalletModel.fromJson(response.data!);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            'Failed to confirm purchase',
        statusCode: e.response?.statusCode,
      );
    }
  }

  @override
  Future<void> withdraw({
    required int amount,
    required String method,
  }) async {
    try {
      await _dio.post<void>(
        '/wallet/withdraw',
        data: {'amount': amount, 'method': method},
      );
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            'Withdrawal failed',
        statusCode: e.response?.statusCode,
      );
    }
  }
}
