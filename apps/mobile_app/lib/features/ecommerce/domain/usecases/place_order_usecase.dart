import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/order_entity.dart';
import '../repositories/ecommerce_repository.dart';

class PlaceOrderParams {
  const PlaceOrderParams({
    required this.shippingAddress,
    required this.paymentMethod,
  });
  final BuyerInfoEntity shippingAddress;
  final String paymentMethod;
}

class PlaceOrderUseCase implements UseCase<OrderEntity, PlaceOrderParams> {
  const PlaceOrderUseCase(this._repository);
  final EcommerceRepository _repository;

  @override
  Future<Either<Failure, OrderEntity>> call(PlaceOrderParams params) async {
    return _repository.placeOrder(
      shippingAddress: params.shippingAddress,
      paymentMethod: params.paymentMethod,
    );
  }
}

class GetOrdersParams {
  const GetOrdersParams({this.cursor});
  final String? cursor;
}

class GetOrdersUseCase implements UseCase<PaginatedOrders, GetOrdersParams> {
  const GetOrdersUseCase(this._repository);
  final EcommerceRepository _repository;

  @override
  Future<Either<Failure, PaginatedOrders>> call(
      GetOrdersParams params) async {
    return _repository.getOrders(cursor: params.cursor);
  }
}

class GetOrderUseCase {
  const GetOrderUseCase(this._repository);
  final EcommerceRepository _repository;

  Future<Either<Failure, OrderEntity>> call(String id) async {
    return _repository.getOrder(id);
  }
}

class CancelOrderUseCase {
  const CancelOrderUseCase(this._repository);
  final EcommerceRepository _repository;

  Future<Either<Failure, OrderEntity>> call(String id) async {
    return _repository.cancelOrder(id);
  }
}
