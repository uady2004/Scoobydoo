import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/features/gifts/data/models/gift_model.dart';

abstract interface class GiftRemoteDatasource {
  Future<List<GiftModel>> getGifts();
  Future<void> sendGift({required String giftId, required String targetUserId, required int count});
}

class GiftRemoteDatasourceImpl implements GiftRemoteDatasource {
  GiftRemoteDatasourceImpl(this._dio);
  final Dio _dio;

  @override
  Future<List<GiftModel>> getGifts() async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/gifts');
      final list = r.data?['gifts'] as List? ?? [];
      return list.map((e) => GiftModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to load gifts');
    }
  }

  @override
  Future<void> sendGift({required String giftId, required String targetUserId, required int count}) async {
    try {
      await _dio.post<void>('/gifts/send', data: {
        'gift_id': giftId, 'target_user_id': targetUserId, 'count': count,
      });
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to send gift');
    }
  }
}
