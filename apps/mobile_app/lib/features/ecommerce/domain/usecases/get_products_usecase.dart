import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../repositories/ecommerce_repository.dart';

class GetProductsParams {
  const GetProductsParams({this.category, this.cursor});
  final String? category;
  final String? cursor;
}

class GetProductsUseCase
    implements UseCase<PaginatedProducts, GetProductsParams> {
  const GetProductsUseCase(this._repository);
  final EcommerceRepository _repository;

  @override
  Future<Either<Failure, PaginatedProducts>> call(
      GetProductsParams params) async {
    return _repository.getProducts(
      category: params.category,
      cursor: params.cursor,
    );
  }
}

class SearchProductsParams {
  const SearchProductsParams({required this.query, this.cursor});
  final String query;
  final String? cursor;
}

class SearchProductsUseCase
    implements UseCase<PaginatedProducts, SearchProductsParams> {
  const SearchProductsUseCase(this._repository);
  final EcommerceRepository _repository;

  @override
  Future<Either<Failure, PaginatedProducts>> call(
      SearchProductsParams params) async {
    return _repository.searchProducts(
      query: params.query,
      cursor: params.cursor,
    );
  }
}

class GetProductParams {
  const GetProductParams(this.id);
  final String id;
}

class GetProductUseCase {
  const GetProductUseCase(this._repository);
  final EcommerceRepository _repository;

  Future<Either<Failure, dynamic>> call(GetProductParams params) async {
    return _repository.getProduct(params.id);
  }
}
